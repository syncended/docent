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
	// Active is the name of the active project. Load sets it to the first
	// project's if it names none.
	Active   string    `toml:"active,omitempty" json:"active"`
	Projects []Project `toml:"projects" json:"projects"`
	// Roots is the layout from before projects; Load moves it into a project
	// named DefaultProject.
	Roots   []Root  `toml:"roots,omitempty" json:"-"`
	Agent   Agent   `toml:"agent" json:"agent"`
	Palette Palette `toml:"palette" json:"palette"`
	// Shortcuts are bindings that differ from the default, by command, as
	// accelerators such as "CmdOrCtrl+Shift+Space" (DESIGN.md, Settings).
	Shortcuts map[string]string `toml:"shortcuts,omitempty" json:"shortcuts"`
}

// DefaultProject names the project that roots from before projects move into.
const DefaultProject = "Default"

// Project is a named group of roots; one project is active at a time.
type Project struct {
	Name  string `toml:"name" json:"name"`
	Roots []Root `toml:"roots" json:"roots"`
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
	// Provider is the id of the active provider preset (core/provider).
	Provider string `toml:"provider,omitempty" json:"provider"`
	// Providers holds each provider's own settings, by provider id.
	Providers map[string]ProviderSettings `toml:"providers,omitempty" json:"providers"`
	// Model and BaseURL are the layout from before per-provider settings,
	// when "openai" meant any OpenAI-compatible API; the shell moves them
	// into Providers at startup.
	Model   string `toml:"model,omitempty" json:"-"`
	BaseURL string `toml:"base_url,omitempty" json:"-"`
}

// ProviderSettings is what the user chose for one provider.
type ProviderSettings struct {
	// Model is the model id; empty means the provider's recommended one.
	Model string `toml:"model,omitempty" json:"model"`
	// Effort is "low", "medium" or "high"; empty means Docent's default.
	Effort string `toml:"effort,omitempty" json:"effort"`
	// Endpoint is the id of the region or plan; empty means the default.
	Endpoint string `toml:"endpoint,omitempty" json:"endpoint"`
	// BaseURL overrides the endpoint's address, e.g. for a proxy.
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
	if len(c.Roots) > 0 && len(c.Projects) == 0 {
		c.Projects = []Project{{Name: DefaultProject, Roots: c.Roots}}
	}
	c.Roots = nil
	for _, p := range c.Projects {
		for i := range p.Roots {
			path, err := expandHome(p.Roots[i].Path)
			if err != nil {
				return nil, err
			}
			p.Roots[i].Path = filepath.Clean(path)
		}
	}
	if len(c.Projects) > 0 && ProjectIndex(&c, c.Active) < 0 {
		c.Active = c.Projects[0].Name
	}
	return &c, nil
}

// ActiveProject returns the active project, or an empty one if there are
// no projects.
func ActiveProject(c *Config) Project {
	if i := ProjectIndex(c, c.Active); i >= 0 {
		return c.Projects[i]
	}
	return Project{}
}

// ProjectIndex returns the index of the project named name, or -1.
func ProjectIndex(c *Config, name string) int {
	for i, p := range c.Projects {
		if p.Name == name {
			return i
		}
	}
	return -1
}

// UniqueProjectName returns name, or name with a number appended if a
// project is already called that.
func UniqueProjectName(c *Config, name string) string {
	unique := name
	for n := 2; ProjectIndex(c, unique) >= 0; n++ {
		unique = fmt.Sprintf("%s %d", name, n)
	}
	return unique
}

// Save writes c to path, replacing the file atomically. Paths under the home
// directory are written with a leading "~".
func Save(path string, c *Config) error {
	out := *c
	out.Roots = nil
	out.Projects = make([]Project, len(c.Projects))
	for i, p := range c.Projects {
		roots := make([]Root, len(p.Roots))
		for j, r := range p.Roots {
			r.Path = contractHome(r.Path)
			roots[j] = r
		}
		out.Projects[i] = Project{Name: p.Name, Roots: roots}
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
