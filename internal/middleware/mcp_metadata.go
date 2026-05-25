package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/garrettladley/pkgsite-mcp/internal/xcontext"
	"github.com/garrettladley/pkgsite-mcp/internal/xhttp"
)

const (
	maxMCPMetadataBodyBytes int64 = 64 << 10
)

type mcpJSONRPCRequest struct {
	Method string          `json:"method"`
	ID     json.RawMessage `json:"id"`
	Params json.RawMessage `json:"params"`
}

// MCPRequestMetadata extracts bounded MCP routing metadata before HTTP
// instrumentation names the root server span.
func MCPRequestMetadata(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clearInternalMCPHeaders(r.Header)
		if isMetadataEligibleMCPRequest(r) {
			r = setMCPClientMetadata(r)
			setMCPRequestMetadata(r)
		}
		next.ServeHTTP(w, r)
	})
}

func clearInternalMCPHeaders(header http.Header) {
	header.Del(xhttp.HeaderInternalMCPClientAddress)
	header.Del(xhttp.HeaderInternalMCPClientPort)
	header.Del(xhttp.HeaderInternalMCPMethod)
	header.Del(xhttp.HeaderInternalMCPName)
	header.Del(xhttp.HeaderInternalMCPRequestID)
}

func isMetadataEligibleMCPRequest(r *http.Request) bool {
	return r.Method == http.MethodPost &&
		r.URL.Path == "/mcp" &&
		r.Body != nil &&
		r.ContentLength >= 0 &&
		r.ContentLength <= maxMCPMetadataBodyBytes
}

func setMCPClientMetadata(r *http.Request) *http.Request {
	client := xcontext.MCPClient{}
	if address := clientIP(r); address != "" && address != "unknown" {
		client.Address = address
		r.Header.Set(xhttp.HeaderInternalMCPClientAddress, address)
	}
	if port := clientPort(r.RemoteAddr); port != "" {
		r.Header.Set(xhttp.HeaderInternalMCPClientPort, port)
		if parsed, err := strconv.Atoi(port); err == nil && parsed > 0 {
			client.Port = parsed
		}
	}
	if client.Address == "" && client.Port <= 0 {
		return r
	}
	return r.WithContext(xcontext.WithMCPClient(r.Context(), client))
}

func setMCPRequestMetadata(r *http.Request) {
	method, name, requestID := readMCPRequestMetadata(r)
	if method != "" {
		r.Header.Set(xhttp.HeaderInternalMCPMethod, method)
	}
	if name != "" {
		r.Header.Set(xhttp.HeaderInternalMCPName, name)
	}
	if requestID != "" {
		r.Header.Set(xhttp.HeaderInternalMCPRequestID, requestID)
	}
}

func readMCPRequestMetadata(r *http.Request) (string, string, string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxMCPMetadataBodyBytes+1))
	r.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return "", "", ""
	}
	if int64(len(body)) > maxMCPMetadataBodyBytes {
		return "", "", ""
	}

	var req mcpJSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", "", ""
	}
	method := strings.TrimSpace(req.Method)
	if method == "" {
		return "", "", ""
	}
	return method, safeMCPName(method, req.Params), safeMCPRequestID(req.ID)
}

func clientPort(remoteAddr string) string {
	_, port, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(port)
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

func safeMCPRequestID(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strings.TrimSpace(string(raw))
	default:
		return ""
	}
}
