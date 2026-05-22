package pkgsiteapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	DefaultOpenAPIURL    = "https://pkg.go.dev/v1beta/openapi.yaml"
	DefaultOpenAPIOutput = "internal/pkgsiteapi/openapi.json"

	maxOpenAPISpecBytes = 16 << 20
)

// FetchOpenAPI downloads the pkgsite OpenAPI document and returns the patched
// JSON form used by code generation.
func FetchOpenAPI(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json, application/yaml, text/yaml, */*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch OpenAPI spec: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("fetch OpenAPI spec: unexpected HTTP status %s", resp.Status)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxOpenAPISpecBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read OpenAPI spec: %w", err)
	}
	if len(raw) > maxOpenAPISpecBytes {
		return nil, fmt.Errorf("read OpenAPI spec: response exceeds %d bytes", maxOpenAPISpecBytes)
	}

	patched, err := PatchOpenAPIJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("patch OpenAPI spec: %w", err)
	}
	return patched, nil
}

// PatchOpenAPIJSON applies the local fixes needed before generating the Go
// client from pkg.go.dev's published OpenAPI document.
func PatchOpenAPIJSON(raw []byte) ([]byte, error) {
	var doc map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode JSON OpenAPI document: %w", err)
	}

	patchPathParameters(doc)
	patchOperationIDs(doc)
	patchErrorSchema(doc)

	patched, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode JSON OpenAPI document: %w", err)
	}
	return append(patched, '\n'), nil
}

func patchPathParameters(doc map[string]any) {
	paths, ok := object(doc["paths"])
	if !ok {
		return
	}

	for path, pathValue := range paths {
		if !strings.Contains(path, "{path}") {
			continue
		}
		pathItem, ok := object(pathValue)
		if !ok {
			continue
		}
		for method, operationValue := range pathItem {
			if !isHTTPMethod(method) {
				continue
			}
			operation, ok := object(operationValue)
			if !ok {
				continue
			}
			ensurePathParameter(operation)
		}
	}
}

func patchOperationIDs(doc map[string]any) {
	paths, ok := object(doc["paths"])
	if !ok {
		return
	}

	for _, pathValue := range paths {
		pathItem, ok := object(pathValue)
		if !ok {
			continue
		}
		for method, operationValue := range pathItem {
			if !isHTTPMethod(method) {
				continue
			}
			operation, ok := object(operationValue)
			if !ok {
				continue
			}
			if operation["operationId"] == "getImported-by" {
				operation["operationId"] = "getImportedBy"
			}
		}
	}
}

func patchErrorSchema(doc map[string]any) {
	components, ok := object(doc["components"])
	if !ok {
		return
	}
	schemas, ok := object(components["schemas"])
	if !ok {
		return
	}
	errorSchema, ok := object(schemas["Error"])
	if !ok {
		return
	}
	properties, ok := object(errorSchema["properties"])
	if !ok {
		return
	}
	delete(properties, "err")
}

func ensurePathParameter(operation map[string]any) {
	parameters, _ := array(operation["parameters"])
	for _, parameterValue := range parameters {
		parameter, ok := object(parameterValue)
		if !ok || parameter["name"] != "path" || parameter["in"] != "path" {
			continue
		}
		parameter["required"] = true
		if _, ok := object(parameter["schema"]); !ok {
			parameter["schema"] = map[string]any{"type": "string"}
		}
		return
	}

	operation["parameters"] = append([]any{pathParameter()}, parameters...)
}

func pathParameter() map[string]any {
	return map[string]any{
		"name":        "path",
		"in":          "path",
		"required":    true,
		"description": "Module or package path.",
		"schema": map[string]any{
			"type": "string",
		},
	}
}

func object(value any) (map[string]any, bool) {
	obj, ok := value.(map[string]any)
	return obj, ok
}

func array(value any) ([]any, bool) {
	arr, ok := value.([]any)
	return arr, ok
}

func isHTTPMethod(method string) bool {
	switch method {
	case "get", "put", "post", "delete", "options", "head", "patch", "trace":
		return true
	default:
		return false
	}
}
