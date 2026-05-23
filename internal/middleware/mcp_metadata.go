package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

const (
	HeaderInternalMCPMethod = "X-Pkgsite-Mcp-Method"
	HeaderInternalMCPName   = "X-Pkgsite-Mcp-Name"

	maxMCPMetadataBodyBytes int64 = 64 << 10
)

type mcpJSONRPCRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// MCPRequestMetadata extracts bounded MCP routing metadata before HTTP
// instrumentation names the root server span.
func MCPRequestMetadata(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del(HeaderInternalMCPMethod)
		r.Header.Del(HeaderInternalMCPName)

		if r.Method == http.MethodPost && r.URL.Path == "/mcp" && r.Body != nil && r.ContentLength >= 0 && r.ContentLength <= maxMCPMetadataBodyBytes {
			method, name := readMCPRequestMetadata(r)
			if method != "" {
				r.Header.Set(HeaderInternalMCPMethod, method)
			}
			if name != "" {
				r.Header.Set(HeaderInternalMCPName, name)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func readMCPRequestMetadata(r *http.Request) (string, string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxMCPMetadataBodyBytes+1))
	if err != nil {
		return "", ""
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if int64(len(body)) > maxMCPMetadataBodyBytes {
		return "", ""
	}

	var req mcpJSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", ""
	}
	method := strings.TrimSpace(req.Method)
	if method == "" {
		return "", ""
	}
	return method, safeMCPName(method, req.Params)
}

func safeMCPName(method string, params json.RawMessage) string {
	switch method {
	case "tools/call", "prompts/get":
		var named struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(params, &named); err != nil {
			return ""
		}
		return strings.TrimSpace(named.Name)
	default:
		return ""
	}
}
