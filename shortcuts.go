// Shortcuts: the keyboard commands of the windows and the global hotkey,
// their defaults and the bindings from the config (DESIGN.md, Settings).
//
// A binding is an accelerator in Wails syntax: modifiers CmdOrCtrl, Ctrl,
// Alt and Shift joined with "+" and a key, such as "CmdOrCtrl+Shift+Space".
// The config stores only bindings that differ from the default. The
// frontend matches bindings against key events (frontend/src/keys.ts).

package main

import (
	"fmt"
	"log"
	"runtime"
	"sort"
	"strings"

	"github.com/syncended/docent/core/config"
)

// command is a keyboard command; ID keys it in the config and the frontend,
// Group is the heading the settings screen lists it under.
type command struct {
	ID      string `json:"id"`
	Group   string `json:"group"`
	Label   string `json:"label"`
	Default string `json:"default"`
}

// commands lists the commands in the order the settings screen shows them.
var commands = []command{
	{"search", "Windows", "Search everywhere", "CmdOrCtrl+Shift+Space"},
	{"overview", "Windows", "Open in overview window", "CmdOrCtrl+Enter"},
	{"tree", "Windows", "Project tree", "CmdOrCtrl+1"},
	{"close", "Windows", "Close file or tab", "CmdOrCtrl+W"},
	{"settings", "Windows", "Settings", "CmdOrCtrl+,"},
	{"projects", "Windows", "Switch project", "CmdOrCtrl+P"},
	{"nextResult", "Results and tabs", "Next result", "CmdOrCtrl+Alt+Down"},
	{"prevResult", "Results and tabs", "Previous result", "CmdOrCtrl+Alt+Up"},
	{"nextTab", "Results and tabs", "Next tab", "Ctrl+Tab"},
	{"prevTab", "Results and tabs", "Previous tab", "Ctrl+Shift+Tab"},
	{"save", "Editor", "Save", "CmdOrCtrl+S"},
	{"definition", "Editor", "Go to definition", "CmdOrCtrl+B"},
	{"usages", "Editor", "Find usages", "Alt+F7"},
	{"line", "Editor", "Go to line", "CmdOrCtrl+L"},
	{"find", "Editor", "Find in file", "CmdOrCtrl+F"},
	{"regex", "Text search", "Regular expression", "Alt+R"},
	{"matchCase", "Text search", "Match case", "Alt+C"},
}

// resolveShortcuts returns every command's binding: the config's where it
// has a valid one, the default otherwise.
func resolveShortcuts(cfg *config.Config) map[string]string {
	out := make(map[string]string, len(commands))
	for _, c := range commands {
		out[c.ID] = c.Default
		if s, ok := cfg.Shortcuts[c.ID]; ok {
			if _, err := parseShortcut(s); err == nil {
				out[c.ID] = s
			} else {
				log.Printf("config: shortcut %s = %q: %v; using %s", c.ID, s, err, c.Default)
			}
		}
	}
	return out
}

// shortcutOverrides keeps the bindings of in that differ from the default,
// which is what the config file stores.
func shortcutOverrides(in map[string]string) map[string]string {
	var out map[string]string
	for _, c := range commands {
		s, ok := in[c.ID]
		if !ok || canonicalShortcut(s) == canonicalShortcut(c.Default) {
			continue
		}
		if out == nil {
			out = map[string]string{}
		}
		out[c.ID] = s
	}
	return out
}

// checkShortcuts validates bindings from the settings screen: every command
// bound, no unknown commands, well-formed accelerators and no binding shared
// by two commands.
func checkShortcuts(in map[string]string) error {
	labels := map[string]string{}
	for _, c := range commands {
		labels[c.ID] = c.Label
	}
	for id := range in {
		if _, ok := labels[id]; !ok {
			return fmt.Errorf("unknown shortcut %q", id)
		}
	}
	taken := map[string]string{}
	for _, c := range commands {
		s, ok := in[c.ID]
		if !ok || s == "" {
			return fmt.Errorf("%s has no shortcut", c.Label)
		}
		if _, err := parseShortcut(s); err != nil {
			return fmt.Errorf("%s: %w", c.Label, err)
		}
		key := canonicalShortcut(s)
		if other, dup := taken[key]; dup {
			return fmt.Errorf("%s and %s share %s", other, c.Label, s)
		}
		taken[key] = c.Label
	}
	return nil
}

// shortcut is a parsed accelerator: which modifiers and which key.
type shortcut struct {
	cmd, ctrl, alt, shift bool
	key                   string
}

// namedKeys are the keys a binding can name besides single characters.
var namedKeys = map[string]bool{
	"space": true, "enter": true, "tab": true, "up": true, "down": true, "left": true, "right": true,
	"backspace": true, "delete": true, "home": true, "end": true,
}

// parseShortcut parses an accelerator such as "CmdOrCtrl+Alt+Down". A
// binding needs a modifier other than Shift, unless its key is a function
// key, so that typing is never bound.
func parseShortcut(s string) (shortcut, error) {
	var out shortcut
	parts := strings.Split(s, "+")
	if s == "" || strings.Contains(s, "++") || strings.HasSuffix(s, "+") {
		return out, fmt.Errorf("%q is not a shortcut", s)
	}
	for _, m := range parts[:len(parts)-1] {
		switch strings.ToLower(m) {
		case "cmdorctrl", "cmd":
			out.cmd = true
		case "ctrl":
			out.ctrl = true
		case "alt", "option":
			out.alt = true
		case "shift":
			out.shift = true
		default:
			return out, fmt.Errorf("%q is not a modifier", m)
		}
	}
	key := strings.ToLower(parts[len(parts)-1])
	if !namedKeys[key] && !functionKey(key) && len([]rune(key)) != 1 {
		return out, fmt.Errorf("%q is not a key", parts[len(parts)-1])
	}
	out.key = key
	if !out.cmd && !out.ctrl && !out.alt && !functionKey(key) {
		return out, fmt.Errorf("%s needs Cmd, Ctrl or Alt", s)
	}
	return out, nil
}

// functionKey reports whether key is F1 to F24.
func functionKey(key string) bool {
	if len(key) < 2 || key[0] != 'f' {
		return false
	}
	n := 0
	for _, r := range key[1:] {
		if r < '0' || r > '9' {
			return false
		}
		n = n*10 + int(r-'0')
	}
	return n >= 1 && n <= 24
}

// canonicalShortcut renders s so that bindings that mean the same keys on
// this platform compare equal: on macOS CmdOrCtrl is Cmd and Ctrl stays
// apart; elsewhere both are Ctrl. An invalid binding renders as itself.
func canonicalShortcut(s string) string {
	p, err := parseShortcut(s)
	if err != nil {
		return s
	}
	var mods []string
	if runtime.GOOS == "darwin" {
		if p.cmd {
			mods = append(mods, "cmd")
		}
		if p.ctrl {
			mods = append(mods, "ctrl")
		}
	} else if p.cmd || p.ctrl {
		mods = append(mods, "ctrl")
	}
	if p.alt {
		mods = append(mods, "alt")
	}
	if p.shift {
		mods = append(mods, "shift")
	}
	sort.Strings(mods)
	return strings.Join(append(mods, p.key), "+")
}

// rebindHotkey moves the global hotkey from one binding to another. If the
// system refuses the new one, the old one is registered again.
func rebindHotkey(from, to string) error {
	if canonicalShortcut(from) == canonicalShortcut(to) {
		return nil
	}
	if err := app.GlobalShortcut.Unregister(from); err != nil {
		log.Printf("unregister hotkey %s: %v", from, err)
	}
	if err := app.GlobalShortcut.Register(to, togglePalette); err != nil {
		if again := app.GlobalShortcut.Register(from, togglePalette); again != nil {
			log.Printf("register hotkey %s again: %v", from, again)
		}
		return fmt.Errorf("hotkey %s: %w", to, err)
	}
	return nil
}
