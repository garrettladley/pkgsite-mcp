package model

import "encoding/json"

const DefaultBaseURL = "https://pkg.go.dev/v1beta"

type APIError struct {
	StatusCode int             `json:"statusCode"`
	Status     string          `json:"status"`
	Message    string          `json:"message,omitempty"`
	Body       json.RawMessage `json:"body,omitempty"`
}

type Result struct {
	Summary     map[string]any   `json:"summary,omitempty"`
	Items       []map[string]any `json:"items,omitempty"`
	Pagination  map[string]any   `json:"pagination,omitempty"`
	Raw         any              `json:"raw,omitempty"`
	Error       *APIError        `json:"error,omitempty"`
	UpstreamURL string           `json:"-"`
	FromCache   bool             `json:"-"`
}
