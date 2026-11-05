package main

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/syncended/docent/core/config"
)

// Projects are named groups of code roots; one is active, and the palette,
// the agent and the overview window see only its roots (DESIGN.md, Code
// sources, Switching projects).

var (
	// current is the config as last applied: the active project and the
	// project list shown by the switcher.
	current atomic.Pointer[config.Config]
	// overviewDirty is set while a tab in the overview window has unsaved
	// edits; switching projects asks about them first.
	overviewDirty atomic.Bool
	// configMu serializes changes to the config: each loads it, changes it
	// and applies it.
	configMu sync.Mutex
)

// projectSwitch tells both windows which project became active; From is the
// one that was active before, so the overview window can keep its tabs.
type projectSwitch struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func init() {
	application.RegisterEvent[projectSwitch]("project:switched")
	application.RegisterEvent[string]("overview:switch")
	application.RegisterEvent[string]("palette:projects")
}

// switchProject makes the project named name active. With unsaved edits in
// the overview window it switches nothing: the window comes forward and asks
// to save or discard them, then calls it again. It reports whether the
// switch happened.
func switchProject(name string) (bool, error) {
	configMu.Lock()
	defer configMu.Unlock()
	cfg, err := config.Load(configPath)
	if err != nil {
		return false, err
	}
	if config.ProjectIndex(cfg, name) < 0 {
		return false, fmt.Errorf("no project %q", name)
	}
	if cfg.Active == name {
		return true, nil
	}
	if overviewDirty.Load() {
		hidePalette()
		showOverview()
		app.Event.Emit("overview:switch", name)
		return false, nil
	}
	cfg.Active = name
	return true, applyConfig(cfg)
}

// newProject asks for a folder and creates a project with it as the only
// root, named after it, then switches to it. It returns the project's name,
// "" if the user cancelled, and whether the switch happened.
func newProject(parent *application.WebviewWindow) (string, bool, error) {
	path, err := pickFolder(parent)
	if err != nil || path == "" {
		return "", false, err
	}
	name, first, err := addProject(path)
	if err != nil || first {
		return name, first, err
	}
	switched, err := switchProject(name)
	return name, switched, err
}

// addProject adds a project with path as its root, named after it. The
// first project becomes active at once; first reports that.
func addProject(path string) (name string, first bool, err error) {
	configMu.Lock()
	defer configMu.Unlock()
	cfg, err := config.Load(configPath)
	if err != nil {
		return "", false, err
	}
	name = config.UniqueProjectName(cfg, filepath.Base(path))
	cfg.Projects = append(cfg.Projects, config.Project{Name: name, Roots: []config.Root{{Path: path}}})
	if len(cfg.Projects) == 1 {
		cfg.Active = name
		return name, true, applyConfig(cfg)
	}
	if err := config.Save(configPath, cfg); err != nil {
		return "", false, err
	}
	saved := *current.Load()
	saved.Projects = cfg.Projects
	current.Store(&saved)
	return name, false, nil
}

// addFolder is the first-run "Add folder…": it asks for a folder and adds it
// to the active project, which has no roots yet, or creates a project for it
// when there is none.
func addFolder(parent *application.WebviewWindow) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	if len(cfg.Projects) == 0 {
		_, _, err := newProject(parent)
		return err
	}
	path, err := pickFolder(parent)
	if err != nil || path == "" {
		return err
	}
	configMu.Lock()
	defer configMu.Unlock()
	if cfg, err = config.Load(configPath); err != nil {
		return err
	}
	i := config.ProjectIndex(cfg, cfg.Active)
	if i < 0 {
		cfg.Projects = []config.Project{{Name: filepath.Base(path)}}
		cfg.Active, i = cfg.Projects[0].Name, 0
	}
	cfg.Projects[i].Roots = append(cfg.Projects[i].Roots, config.Root{Path: path})
	return applyConfig(cfg)
}

// showProjects summons the palette with the project list, from the
// overview window (DESIGN.md, Overview window).
func showProjects() { summon("palette:projects") }

// projectNames lists the projects in config order.
func projectNames(cfg *config.Config) []string {
	names := make([]string, len(cfg.Projects))
	for i, p := range cfg.Projects {
		names[i] = p.Name
	}
	return names
}
