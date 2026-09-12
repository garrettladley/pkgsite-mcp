package model

import "encoding/json"

const DefaultBaseURL = "https://pkg.go.dev/v1"

type APIError struct {
	StatusCode int             `json:"statusCode"`
	Status     string          `json:"status"`
	Code       *int            `json:"code,omitempty"`
	Message    string          `json:"message,omitempty"`
	Fixes      []string        `json:"fixes,omitempty"`
	Candidates []Candidate     `json:"candidates,omitempty"`
	RetryAfter string          `json:"retryAfter,omitempty"`
	Body       json.RawMessage `json:"body,omitempty"`
}

type Candidate struct {
	ModulePath  string `json:"modulePath,omitempty"`
	PackagePath string `json:"packagePath,omitempty"`
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
