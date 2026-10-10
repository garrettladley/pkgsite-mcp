package skills

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCatalogManifests(t *testing.T) {
	t.Parallel()
	catalog, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 5 {
		t.Fatalf("catalog has %d skills, want 5", len(catalog))
	}
	for _, skill := range catalog {
		t.Run(string(skill.Name), func(t *testing.T) {
			t.Parallel()
			entry := skill.entry()
			if len(entry.Resources) != 1 || entry.Resources[0].URI != entry.URI {
				t.Fatalf("manifest = %+v", entry)
			}
			resource := entry.Resources[0]
			digest := sha256.Sum256([]byte(skill.Document))
			if resource.Size != len(skill.Document) || resource.Digest != fmt.Sprintf("sha256:%x", digest) {
				t.Fatalf("manifest does not match bytes: %+v", resource)
			}
			if strings.HasPrefix(skill.Content, "---") || !strings.HasPrefix(skill.Content, "# ") {
				t.Fatalf("compatibility content contains frontmatter: %q", skill.Content)
			}
			for _, related := range skill.Related {
				if _, err := Load(string(related)); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestParseSkillPreservesFrontmatter(t *testing.T) {
	t.Parallel()
	document := "---\nname: example\ndescription: Example guide.\nlicense: MIT\nmetadata:\n  author: pkgsite\n---\n\n# Example\n"
	skill, err := parseSkill("example", document)
	if err != nil {
		t.Fatal(err)
	}
	var frontmatter map[string]any
	if err := json.Unmarshal(skill.Frontmatter, &frontmatter); err != nil {
		t.Fatal(err)
	}
	if frontmatter["license"] != "MIT" || frontmatter["metadata"].(map[string]any)["author"] != "pkgsite" {
		t.Fatalf("lost authored frontmatter: %+v", skill.Frontmatter)
	}
	if skill.Document != document || skill.Content != "# Example" {
		t.Fatalf("document/body altered: %+v", skill)
	}
}

func TestNativePaginationAndDirectLookup(t *testing.T) {
	t.Parallel()
	catalog, err := List()
	if err != nil {
		t.Fatal(err)
	}
	s := &service{catalog: catalog, pageSize: 2}
	meta := mcp.Meta{
		mcp.MetaKeyProtocolVersion:    "2026-07-28",
		mcp.MetaKeyClientCapabilities: &mcp.ClientCapabilities{},
	}
	var entries []Entry
	cursor := ""
	for {
		page, err := s.list(t.Context(), nil, &listParams{Meta: meta, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Skills) > 2 || page.ResultType != "complete" || page.TTLMs == nil || page.CacheScope != "public" {
			t.Fatalf("invalid page: %+v", page)
		}
		entries = append(entries, page.Skills...)
		if page.NextCursor == "" {
			break
		}
		if page.NextCursor == cursor {
			t.Fatal("cursor did not advance")
		}
		cursor = page.NextCursor
	}
	if len(entries) != len(catalog) {
		t.Fatalf("got %d entries, want %d", len(entries), len(catalog))
	}
	for i, skill := range catalog {
		if !reflect.DeepEqual(entries[i], skill.entry()) {
			t.Fatalf("entry %d skipped or duplicated", i)
		}
	}
	// Get a skill that was not on the first page, without completing a listing.
	got, err := s.get(t.Context(), nil, &getParams{Meta: meta, URI: catalog[4].URI})
	if err != nil || got == nil || !reflect.DeepEqual(got.Skill, catalog[4].entry()) {
		t.Fatalf("direct lookup = %+v, %v", got, err)
	}
	_, err = s.list(t.Context(), nil, &listParams{Meta: meta, Cursor: "invalid!"})
	assertRPCError(t, err, jsonrpc.CodeInvalidParams)
	_, err = s.get(t.Context(), nil, &getParams{Meta: meta, URI: "skill://pkgsite/missing/SKILL.md"})
	assertRPCError(t, err, jsonrpc.CodeInvalidParams)
}

func TestLegacyNativeEnvelope(t *testing.T) {
	t.Parallel()
	catalog, err := List()
	if err != nil {
		t.Fatal(err)
	}
	s := &service{catalog: catalog, pageSize: 2}
	result, err := s.list(t.Context(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Skills) != 2 || result.ResultType != "" || result.TTLMs != nil || result.CacheScope != "" {
		t.Fatalf("legacy envelope = %+v", result)
	}
}

func assertRPCError(t *testing.T, err error, code int64) {
	t.Helper()
	var rpcError *jsonrpc.Error
	if !errors.As(err, &rpcError) || rpcError.Code != code {
		t.Fatalf("error = %v, want JSON-RPC code %d", err, code)
	}
}
