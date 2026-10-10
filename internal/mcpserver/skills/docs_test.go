package skills

import (
	"reflect"
	"testing"
)

func TestCatalogReturnsIndependentCopies(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"list", "load"} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			before := loadSkillForTest(t, string(Entities))
			var skill Skill
			if source == "list" {
				catalog, err := List()
				if err != nil {
					t.Fatal(err)
				}
				skill = catalog[0]
				catalog[0] = Skill{}
			} else {
				skill = loadSkillForTest(t, string(Entities))
			}
			skill.Related[0] = "changed"
			skill.Frontmatter[0] = '!'

			after := loadSkillForTest(t, string(Entities))
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("%s caller mutated cached skill: %+v", source, after)
			}
			catalog, err := List()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(catalog[0], before) {
				t.Fatalf("%s caller mutated cached listing: %+v", source, catalog[0])
			}
		})
	}
}

func TestSkillEntryReturnsIndependentCopies(t *testing.T) {
	t.Parallel()
	document := "---\nname: example\ndescription: Example guide.\nextra:\n  - label: original\n---\n\n# Example\n"
	skill, err := parseSkill("example", document)
	if err != nil {
		t.Fatal(err)
	}
	before := skill.entry()
	entry := skill.entry()
	entry.Frontmatter[0] = '!'
	entry.Resources[0].Digest = "changed"
	if after := skill.entry(); !reflect.DeepEqual(after, before) {
		t.Fatalf("native result mutated cached entry: %+v", after)
	}
}

func TestLoadParsesHeader(t *testing.T) {
	t.Parallel()

	skill := loadSkillForTest(t, "pkgsite/overview")
	tests := []struct {
		name  string
		check func(*testing.T)
	}{
		{name: "name", check: func(t *testing.T) {
			t.Helper()
			if skill.Name != "pkgsite/overview" {
				t.Fatalf("Name = %q", skill.Name)
			}
		}},
		{name: "description", check: func(t *testing.T) {
			t.Helper()
			if skill.Description == "" {
				t.Fatal("Description is empty")
			}
		}},
		{name: "related", check: func(t *testing.T) {
			t.Helper()
			if len(skill.Related) == 0 {
				t.Fatal("Related is empty")
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.check(t)
		})
	}
}

func loadSkillForTest(t *testing.T, name string) Skill {
	t.Helper()

	skill, err := Load(name)
	if err != nil {
		t.Fatal(err)
	}
	return skill
}
