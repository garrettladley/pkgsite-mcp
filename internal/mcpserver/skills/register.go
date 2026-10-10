package skills

import (
	"context"
	"encoding/base64"
	"fmt"
	"path"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const Extension = "io.modelcontextprotocol/skills"

type listParams struct {
	mcp.ParamsBase
	Cursor string `json:"cursor,omitempty"`
}

type getParams struct {
	mcp.ParamsBase
	URI string `json:"uri"`
}

// The extension envelope is explicit because the released SDK does not yet
// provide Skills types. Legacy backports omit fields introduced in 2026-07-28.
type resultEnvelope struct {
	mcp.ResultBase
	ResultType string `json:"resultType,omitempty"`
	TTLMs      *int   `json:"ttlMs,omitempty"`
	CacheScope string `json:"cacheScope,omitempty"`
}

type listResult struct {
	resultEnvelope
	Skills     []Entry `json:"skills"`
	NextCursor string  `json:"nextCursor,omitempty"`
}

type getResult struct {
	resultEnvelope
	Skill Entry `json:"skill"`
}

var (
	_ mcp.Params = (*listParams)(nil)
	_ mcp.Params = (*getParams)(nil)
	_ mcp.Result = (*listResult)(nil)
	_ mcp.Result = (*getResult)(nil)
)

type service struct {
	catalog  []Skill
	pageSize int
}

// Register serves the embedded catalog through the Skills extension and normal
// resources. The caller must advertise Extension in its server capabilities.
func Register(server *mcp.Server) error {
	catalog, err := cachedCatalog()
	if err != nil {
		return err
	}
	s := &service{catalog: catalog, pageSize: mcp.DefaultPageSize}
	if err := mcp.AddReceivingCustomMethod(server, "skills/list", s.list); err != nil {
		return err
	}
	if err := mcp.AddReceivingCustomMethod(server, "skills/get", s.get); err != nil {
		return err
	}
	// Custom-method registration permits absent params. Normalize our concrete
	// parameter pointers before the SDK's metadata accessors and tracing run.
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			switch req := req.(type) {
			case *mcp.ServerRequest[*listParams]:
				if req.Params == nil {
					req.Params = &listParams{}
				}
			case *mcp.ServerRequest[*getParams]:
				if req.Params == nil {
					req.Params = &getParams{}
				}
			}
			return next(ctx, method, req)
		}
	})
	for _, skill := range catalog {
		server.AddResource(&mcp.Resource{
			URI: skill.URI, Name: path.Base(string(skill.Name)),
			Description: skill.Description, MIMEType: "text/markdown",
		}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
				URI: skill.URI, MIMEType: "text/markdown", Text: skill.Document,
			}}}, nil
		})
	}
	return nil
}

func (s *service) list(_ context.Context, session *mcp.ServerSession, params *listParams) (*listResult, error) {
	if params == nil {
		params = &listParams{}
	}
	req := &mcp.ServerRequest[*listParams]{Session: session, Params: params}
	start := 0
	if params.Cursor != "" {
		uri, err := base64.RawURLEncoding.DecodeString(params.Cursor)
		if err != nil {
			return nil, invalidParams("invalid skill cursor")
		}
		index := slices.IndexFunc(s.catalog, func(skill Skill) bool { return skill.URI == string(uri) })
		if index < 0 {
			return nil, invalidParams("unknown skill cursor")
		}
		start = index + 1
	}
	end := min(start+s.pageSize, len(s.catalog))
	result := &listResult{resultEnvelope: envelope(req.ProtocolVersion()), Skills: make([]Entry, 0, end-start)}
	for _, skill := range s.catalog[start:end] {
		result.Skills = append(result.Skills, skill.entry())
	}
	if end < len(s.catalog) {
		result.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(s.catalog[end-1].URI))
	}
	return result, nil
}

func (s *service) get(_ context.Context, session *mcp.ServerSession, params *getParams) (*getResult, error) {
	if params == nil {
		return nil, invalidParams("missing skill URI")
	}
	req := &mcp.ServerRequest[*getParams]{Session: session, Params: params}
	for _, skill := range s.catalog {
		if skill.URI == params.URI {
			return &getResult{resultEnvelope: envelope(req.ProtocolVersion()), Skill: skill.entry()}, nil
		}
	}
	return nil, invalidParams(fmt.Sprintf("unknown skill URI %q", params.URI))
}

func envelope(protocolVersion string) resultEnvelope {
	if protocolVersion < "2026-07-28" {
		return resultEnvelope{}
	}
	return resultEnvelope{ResultType: "complete", TTLMs: new(int), CacheScope: "public"}
}

func invalidParams(message string) error {
	return &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: message}
}
