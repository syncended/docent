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
		Roots: []Root{
			{Path: filepath.Join(home, "src", "app"), Include: []string{"api", "proto"}},
			{Path: filepath.Join(string(filepath.Separator), "opt", "code")},
		},
		Agent: Agent{Provider: "anthropic", Model: "claude-sonnet-5-5"},
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
	if out.Roots[0].Path != in.Roots[0].Path || !slices.Equal(out.Roots[0].Include, in.Roots[0].Include) ||
		out.Roots[1].Path != in.Roots[1].Path || out.Agent != in.Agent {
		t.Errorf("round trip: got %+v, want %+v", out, in)
	}
}
