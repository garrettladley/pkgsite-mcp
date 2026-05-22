package pkgsiteapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFetchOpenAPI(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://example.test/openapi.json" {
				t.Fatalf("url = %s, want https://example.test/openapi.json", req.URL.String())
			}
			if got := req.Header.Get("Accept"); !strings.Contains(got, "application/json") {
				t.Fatalf("Accept = %q, want application/json", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body: io.NopCloser(strings.NewReader(`{
					"paths": {
						"/package/{path}": {
							"get": {"operationId": "getPackage"}
						}
					}
				}`)),
			}, nil
		}),
	}

	patched, err := FetchOpenAPI(t.Context(), client, "https://example.test/openapi.json")
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]any
	if err := json.Unmarshal(patched, &doc); err != nil {
		t.Fatal(err)
	}
	assertSingleRequiredPathParameter(t, operation(t, doc, "/package/{path}"))
}

func TestPatchOpenAPIJSON(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"openapi": "3.0.3",
		"paths": {
			"/imported-by/{path}": {
				"get": {
					"operationId": "getImported-by",
					"parameters": [
						{"name": "module", "in": "query", "schema": {"type": "string"}}
					]
				}
			},
			"/module/{path}": {
				"get": {
					"operationId": "getModule",
					"parameters": [
						{"name": "path", "in": "path", "required": false, "schema": {"type": "string"}}
					]
				}
			},
			"/search": {
				"get": {
					"operationId": "getSearch"
				}
			}
		},
		"components": {
			"schemas": {
				"Error": {
					"type": "object",
					"properties": {
						"err": {"$ref": "#/components/schemas/error"},
						"message": {"type": "string"}
					}
				}
			}
		}
	}`)

	patched, err := PatchOpenAPIJSON(raw)
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]any
	if err := json.Unmarshal(patched, &doc); err != nil {
		t.Fatal(err)
	}

	importedBy := operation(t, doc, "/imported-by/{path}")
	if got := importedBy["operationId"]; got != "getImportedBy" {
		t.Fatalf("operationId = %v, want getImportedBy", got)
	}
	assertSingleRequiredPathParameter(t, importedBy)

	module := operation(t, doc, "/module/{path}")
	assertSingleRequiredPathParameter(t, module)

	search := operation(t, doc, "/search")
	if _, ok := search["parameters"]; ok {
		t.Fatal("search operation unexpectedly received parameters")
	}

	errorProperties := doc["components"].(map[string]any)["schemas"].(map[string]any)["Error"].(map[string]any)["properties"].(map[string]any)
	if _, ok := errorProperties["err"]; ok {
		t.Fatal("Error.properties.err was not removed")
	}
}

func operation(t *testing.T, doc map[string]any, path string) map[string]any {
	t.Helper()

	paths := doc["paths"].(map[string]any)
	pathItem := paths[path].(map[string]any)
	return pathItem["get"].(map[string]any)
}

func assertSingleRequiredPathParameter(t *testing.T, operation map[string]any) {
	t.Helper()

	parameters := operation["parameters"].([]any)
	count := 0
	for _, value := range parameters {
		parameter := value.(map[string]any)
		if parameter["name"] == "path" && parameter["in"] == "path" {
			count++
			if parameter["required"] != true {
				t.Fatalf("path parameter required = %v, want true", parameter["required"])
			}
		}
	}
	if count != 1 {
		t.Fatalf("path parameter count = %d, want 1", count)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
