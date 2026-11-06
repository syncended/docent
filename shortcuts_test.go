package main

import (
	"runtime"
	"strings"
	"testing"

	"github.com/syncended/docent/core/config"
)

func TestParseShortcut(t *testing.T) {
	valid := []string{"CmdOrCtrl+Shift+Space", "Alt+F7", "F12", "Ctrl+Tab", "CmdOrCtrl+,", "Cmd+Alt+Down", "Shift+Alt+/"}
	for _, s := range valid {
		if _, err := parseShortcut(s); err != nil {
			t.Errorf("parseShortcut(%q): %v", s, err)
		}
	}
	invalid := []string{"", "B", "Shift+B", "Ctrl+", "Ctrl++", "Hyper+B", "Ctrl+Foo", "F25", "Ctrl+Space+B"}
	for _, s := range invalid {
		if _, err := parseShortcut(s); err == nil {
			t.Errorf("parseShortcut(%q) accepted", s)
		}
	}
}

func TestCanonicalShortcut(t *testing.T) {
	if runtime.GOOS == "darwin" {
		if canonicalShortcut("CmdOrCtrl+B") == canonicalShortcut("Ctrl+B") {
			t.Error("Cmd and Ctrl compare equal on macOS")
		}
	} else if canonicalShortcut("CmdOrCtrl+B") != canonicalShortcut("Ctrl+B") {
		t.Error("CmdOrCtrl and Ctrl differ outside macOS")
	}
	if canonicalShortcut("Shift+Alt+b") != canonicalShortcut("Alt+Shift+B") {
		t.Error("modifier order and case matter")
	}
}

func defaults() map[string]string {
	out := map[string]string{}
	for _, c := range commands {
		out[c.ID] = c.Default
	}
	return out
}

func TestCheckShortcuts(t *testing.T) {
	if err := checkShortcuts(defaults()); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	for name, change := range map[string]func(map[string]string){
		"shared":  func(m map[string]string) { m["find"] = m["definition"] },
		"missing": func(m map[string]string) { delete(m, "find") },
		"empty":   func(m map[string]string) { m["find"] = "" },
		"unknown": func(m map[string]string) { m["fly"] = "Alt+F" },
		"typing":  func(m map[string]string) { m["find"] = "Shift+F" },
	} {
		m := defaults()
		change(m)
		if err := checkShortcuts(m); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	m := defaults()
	m["find"] = "CmdOrCtrl+Alt+Down"
	if err := checkShortcuts(m); err == nil || !strings.Contains(err.Error(), "Next result") || !strings.Contains(err.Error(), "Find in file") {
		t.Errorf("shared binding error = %v", err)
	}
}

func TestResolveAndOverrides(t *testing.T) {
	cfg := &config.Config{Shortcuts: map[string]string{"search": "Alt+Space", "find": "nonsense", "nope": "Alt+X"}}
	got := resolveShortcuts(cfg)
	if got["search"] != "Alt+Space" || got["find"] != "CmdOrCtrl+F" || len(got) != len(commands) {
		t.Errorf("resolveShortcuts = %v", got)
	}
	over := shortcutOverrides(got)
	if len(over) != 1 || over["search"] != "Alt+Space" {
		t.Errorf("shortcutOverrides = %v", over)
	}
	if shortcutOverrides(defaults()) != nil {
		t.Error("defaults leave overrides")
	}
}
