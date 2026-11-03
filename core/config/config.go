// Package config loads Docent's user configuration.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the user configuration, stored as TOML.
type Config struct {
	Roots   []Root  `toml:"roots" json:"roots"`
	Agent   Agent   `toml:"agent" json:"agent"`
	Palette Palette `toml:"palette" json:"palette"`
}

// Palette configures the search palette window.
type Palette struct {
	// Screen is where the palette opens: "focus" (the screen with the focused
	// window, the default), "mouse" (the screen with the pointer) or "primary".
	Screen string `toml:"screen,omitempty" json:"screen"`
}

// Root is a local checkout to index.
type Root struct {
	// Path to the checkout. A leading "~" is expanded to the home directory.
	Path string `toml:"path" json:"path"`
	// Include limits indexing to these subdirectories (relative to Path).
	// Empty means the whole checkout.
	Include []string `toml:"include,omitempty" json:"include"`
	// Exclude lists doublestar glob patterns (relative to Path) to skip.
	Exclude []string `toml:"exclude,omitempty" json:"exclude"`
}

// Agent configures the built-in agent.
type Agent struct {
	// Provider is "anthropic" or "openai" (any OpenAI-compatible API).
	Provider string `toml:"provider,omitempty" json:"provider"`
	Model    string `toml:"model,omitempty" json:"model"`
	// BaseURL overrides the provider's default endpoint.
	BaseURL string `toml:"base_url,omitempty" json:"baseUrl"`
}

// DefaultPath returns the config file location: $DOCENT_CONFIG if set,
// otherwise <user config dir>/docent/config.toml.
func DefaultPath() (string, error) {
	if p := os.Getenv("DOCENT_CONFIG"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "docent", "config.toml"), nil
}

// Load reads the config at path. A missing file yields an empty config.
func Load(path string) (*Config, error) {
	var c Config
	_, err := toml.DecodeFile(path, &c)
	if errors.Is(err, fs.ErrNotExist) {
		return &c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	for i := range c.Roots {
		p, err := expandHome(c.Roots[i].Path)
		if err != nil {
			return nil, err
		}
		c.Roots[i].Path = filepath.Clean(p)
	}
	return &c, nil
}

// Save writes c to path, replacing the file atomically. Paths under the home
// directory are written with a leading "~".
func Save(path string, c *Config) error {
	out := *c
	out.Roots = make([]Root, len(c.Roots))
	for i, r := range c.Roots {
		r.Path = contractHome(r.Path)
		out.Roots[i] = r
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := toml.NewEncoder(tmp).Encode(&out); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func contractHome(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if rel, err := filepath.Rel(home, p); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return "~/" + filepath.ToSlash(rel)
	}
	return p
}

func expandHome(p string) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~")), nil
}
