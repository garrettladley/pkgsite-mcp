package skills

import "testing"

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
