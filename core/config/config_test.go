package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSaveLoad(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "docent", "config.toml")
	in := &Config{
		Active: "Home",
		Projects: []Project{
			{Name: "Work", Roots: []Root{{Path: filepath.Join(home, "src", "app"), Include: []string{"api", "proto"}}}},
			{Name: "Home", Roots: []Root{{Path: filepath.Join(string(filepath.Separator), "opt", "code")}}},
		},
		Agent:     Agent{Provider: "anthropic", Model: "claude-sonnet-5-5"},
		Shortcuts: map[string]string{"search": "Alt+Space"},
	}
	if err := Save(path, in); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), `path = "~/src/app"`) {
		t.Errorf("home not contracted:\n%s", raw)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Projects) != 2 || out.Active != "Home" || out.Agent != in.Agent || out.Shortcuts["search"] != "Alt+Space" {
		t.Fatalf("round trip: got %+v, want %+v", out, in)
	}
	work, home2 := out.Projects[0], out.Projects[1]
	if work.Name != "Work" || work.Roots[0].Path != in.Projects[0].Roots[0].Path || !slices.Equal(work.Roots[0].Include, in.Projects[0].Roots[0].Include) ||
		home2.Roots[0].Path != in.Projects[1].Roots[0].Path {
		t.Errorf("round trip: got %+v, want %+v", out.Projects, in.Projects)
	}
	if got := ActiveProject(out).Name; got != "Home" {
		t.Errorf("ActiveProject = %q", got)
	}
}

func TestLoadMigratesRoots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	old := "[[roots]]\npath = \"/opt/code\"\ninclude = [\"api\"]\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Roots != nil || len(c.Projects) != 1 || c.Active != DefaultProject {
		t.Fatalf("migration: got %+v", c)
	}
	if p := c.Projects[0]; p.Name != DefaultProject || len(p.Roots) != 1 || filepath.ToSlash(p.Roots[0].Path) != "/opt/code" || !slices.Equal(p.Roots[0].Include, []string{"api"}) {
		t.Errorf("migrated project = %+v", p)
	}
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "[[roots]]") {
		t.Errorf("saved config keeps the old roots:\n%s", raw)
	}
}

func TestActiveFallsBackToFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := Save(path, &Config{Active: "Gone", Projects: []Project{{Name: "A"}, {Name: "B"}}}); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Active != "A" {
		t.Errorf("Active = %q, want A", c.Active)
	}
}

func TestUniqueProjectName(t *testing.T) {
	c := &Config{Projects: []Project{{Name: "app"}, {Name: "app 2"}}}
	if got := UniqueProjectName(c, "app"); got != "app 3" {
		t.Errorf("UniqueProjectName(app) = %q", got)
	}
	if got := UniqueProjectName(c, "lib"); got != "lib" {
		t.Errorf("UniqueProjectName(lib) = %q", got)
	}
}
