package skills

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed docs/*/SKILL.md
var documents embed.FS

var cachedCatalog = sync.OnceValues(readCatalog)

type Name string

const (
	Overview   Name = "pkgsite/overview"
	Entities   Name = "pkgsite/entities"
	Operations Name = "pkgsite/operations"
	Pagination Name = "pkgsite/pagination"
	Precision  Name = "pkgsite/precision"
)

// Skill carries both the compatibility tool view and the native resource bytes.
type Skill struct {
	Name        Name
	Description string
	Related     []Name
	Content     string
	URI         string
	Document    string
	Frontmatter json.RawMessage
	resource    Resource
}

const (
	FieldName        = "name"
	FieldDescription = "description"
	FieldRelated     = "related"
)

type Resource struct {
	URI    string `json:"uri"`
	Digest string `json:"digest"`
	Size   int    `json:"size"`
}

type Entry struct {
	URI         string          `json:"uri"`
	Frontmatter json.RawMessage `json:"frontmatter"`
	Resources   []Resource      `json:"resources"`
}

func (s Skill) entry() Entry {
	return Entry{
		URI: s.URI, Frontmatter: slices.Clone(s.Frontmatter),
		Resources: []Resource{s.resource},
	}
}

// List returns independent copies of the immutable embedded catalog.
func List() ([]Skill, error) {
	catalog, err := cachedCatalog()
	if err != nil {
		return nil, err
	}
	skills := make([]Skill, len(catalog))
	for i, skill := range catalog {
		skills[i] = skill.clone()
	}
	return skills, nil
}

func readCatalog() ([]Skill, error) {
	files, err := fs.Glob(documents, "docs/*/SKILL.md")
	if err != nil {
		return nil, err
	}
	catalog := make([]Skill, 0, len(files))
	for _, file := range files {
		data, err := documents.ReadFile(file)
		if err != nil {
			return nil, err
		}
		skill, err := parseSkill(path.Base(path.Dir(file)), string(data))
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", file, err)
		}
		catalog = append(catalog, skill)
	}
	return catalog, nil
}

func parseSkill(name, document string) (Skill, error) {
	rest, ok := strings.CutPrefix(document, "---\n")
	if !ok {
		return Skill{}, fmt.Errorf("missing YAML frontmatter")
	}
	header, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return Skill{}, fmt.Errorf("missing frontmatter delimiter")
	}
	var frontmatter map[string]any
	if err := yaml.Unmarshal([]byte(header), &frontmatter); err != nil {
		return Skill{}, err
	}
	// Encode once so all authored fields pass through the wire unchanged.
	encoded, err := json.Marshal(frontmatter)
	if err != nil {
		return Skill{}, fmt.Errorf("frontmatter is not JSON-compatible: %w", err)
	}
	if frontmatter["name"] != name {
		return Skill{}, fmt.Errorf("frontmatter name must match directory %q", name)
	}
	description, ok := frontmatter["description"].(string)
	if !ok || strings.TrimSpace(description) == "" {
		return Skill{}, fmt.Errorf("missing description")
	}
	legacyName := Name("pkgsite/" + name)
	skill := Skill{
		Name: legacyName, Description: description, Content: strings.TrimSpace(body),
		URI: "skill://" + string(legacyName) + "/SKILL.md", Document: document, Frontmatter: encoded,
	}
	digest := sha256.Sum256([]byte(document))
	skill.resource = Resource{URI: skill.URI, Digest: fmt.Sprintf("sha256:%x", digest), Size: len(document)}
	if metadata, ok := frontmatter["metadata"].(map[string]any); ok {
		if related, ok := metadata["related-skills"].(string); ok {
			for item := range strings.SplitSeq(related, ",") {
				if item = strings.TrimSpace(item); item != "" {
					skill.Related = append(skill.Related, Name(item))
				}
			}
		}
	}
	return skill, nil
}

func Load(name string) (Skill, error) {
	catalog, err := cachedCatalog()
	if err != nil {
		return Skill{}, err
	}
	for _, skill := range catalog {
		if string(skill.Name) == name {
			return skill.clone(), nil
		}
	}
	return Skill{}, fmt.Errorf("unknown pkgsite skill %q", name)
}

func (s Skill) clone() Skill {
	s.Related = slices.Clone(s.Related)
	s.Frontmatter = slices.Clone(s.Frontmatter)
	return s
}
