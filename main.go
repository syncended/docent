// Command docent is the desktop shell: a background app with a global hotkey
// that shows the search palette over the active window.
package main

import (
	"context"
	"embed"
	"errors"
	"log"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/syncended/docent/core/code"
	"github.com/syncended/docent/core/config"
)

//go:embed all:frontend/dist
var assets embed.FS

// hotkey summons the palette (DESIGN.md, Summoning).
const hotkey = "CmdOrCtrl+Shift+Space"

var (
	app           *application.App
	palette       *application.WebviewWindow
	configPath    string
	paletteScreen atomic.Value // config.Palette.Screen
	cacheDir      string
	layout        atomic.Value // current layout name, see layouts

	indexing       atomic.Bool
	reindexPending atomic.Bool
	cancelReindex  atomic.Pointer[context.CancelFunc]

	// dialogOpen keeps the palette visible while a system dialog has focus.
	dialogOpen atomic.Bool
)

// layouts are the palette window sizes: a Spotlight-like search bar, the bar
// with results below it, and the file viewer.
var layouts = map[string]struct{ width, height int }{
	"compact": {720, 92},
	"results": {720, 600},
	"viewer":  {1100, 740},
}

func init() {
	application.RegisterEvent[code.Progress]("index:progress")
	application.RegisterEvent[[]code.RootInfo]("index:done")
	application.RegisterEvent[string]("palette:shown")
	application.RegisterEvent[string]("settings:open")
	application.RegisterEvent[agentEvent]("agent:event")
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var err error
	if configPath, err = config.DefaultPath(); err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	paletteScreen.Store(cfg.Palette.Screen)
	if cacheDir, err = code.DefaultCacheDir(); err != nil {
		return err
	}
	if err := code.Open(cfg, cacheDir); err != nil {
		return err
	}
	defer code.Close()

	app = application.New(application.Options{
		Name:        "Docent",
		Description: "Read-mostly IDE for searching and viewing code with an agent",
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(assets),
			Middleware: apiMiddleware,
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
	})

	// A non-activating floating panel, like Spotlight: it takes keyboard
	// input without activating Docent, so focus stays with the previous app.
	palette = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "palette",
		Width:     layouts["compact"].width,
		Height:    layouts["compact"].height,
		Frameless: true,
		Hidden:    true,
		URL:       "/",
		// Mac.WindowLevel takes precedence on macOS.
		AlwaysOnTop: true,
		Mac: application.MacWindow{
			CornerRadius: 16,
			// The palette is floating bubbles over a transparent window; the
			// webview is made transparent by the private_mac_apis build tag.
			Backdrop:    application.MacBackdropTransparent,
			WindowClass: application.MacWindowClassPanel,
			PanelPreferences: application.MacPanelPreferences{
				FloatingPanel: true,
				NonActivating: true,
			},
			WindowLevel:        application.MacWindowLevelFloating,
			CollectionBehavior: application.MacWindowCollectionBehaviorCanJoinAllSpaces | application.MacWindowCollectionBehaviorFullScreenAuxiliary,
		},
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: true,
			// The frontend draws its own bubbles; no shadow around the
			// transparent window.
			DisableFramelessWindowDecorations: true,
		},
		// Transparent, so the rounded corners drawn by the frontend show through.
		BackgroundType: application.BackgroundTypeTransparent,
	})

	// Like Spotlight: clicking elsewhere dismisses the palette, except while
	// a dialog opened from it has focus.
	palette.OnWindowEvent(events.Common.WindowLostFocus, func(*application.WindowEvent) {
		if !dialogOpen.Load() {
			hidePalette()
		}
	})

	tray := app.SystemTray.New()
	tray.SetLabel("⌕")
	tray.SetTooltip("Docent")
	menu := app.NewMenu()
	menu.Add("Search…").OnClick(func(*application.Context) { showPalette() })
	menu.Add("Settings…").OnClick(func(*application.Context) { showSettings() })
	menu.Add("Reindex").OnClick(func(*application.Context) { go reindex() })
	menu.AddSeparator()
	menu.Add("Quit Docent").OnClick(func(*application.Context) { app.Quit() })
	tray.SetMenu(menu)

	if err := app.GlobalShortcut.Register(hotkey, togglePalette); err != nil {
		log.Printf("register hotkey %s: %v", hotkey, err)
	}

	go reindex()
	return app.Run()
}

func togglePalette() {
	if palette.IsVisible() {
		hidePalette()
		return
	}
	showPalette()
}

func showPalette() {
	placeOn(targetScreen())
	palette.Show()
	palette.Focus()
	app.Event.Emit("palette:shown", "")
}

func showSettings() {
	showPalette()
	app.Event.Emit("settings:open", "")
}

func hidePalette() {
	palette.Hide()
}

// pickFolder asks for a directory with the system dialog, attached to the
// palette. It returns "" if the user cancels.
func pickFolder() (string, error) {
	dialogOpen.Store(true)
	defer func() {
		dialogOpen.Store(false)
		palette.Focus()
	}()
	return app.Dialog.OpenFile().
		SetTitle("Add code root").
		CanChooseFiles(false).
		CanChooseDirectories(true).
		CanCreateDirectories(false).
		AttachToWindow(palette).
		PromptForSingleSelection()
}

// applyConfig saves cfg, reopens the index with its roots and re-indexes in
// the background.
func applyConfig(cfg *config.Config) error {
	if err := config.Save(configPath, cfg); err != nil {
		return err
	}
	// Reload what was saved, so paths are normalized the same way as at startup.
	saved, err := config.Load(configPath)
	if err != nil {
		return err
	}
	if cancel := cancelReindex.Load(); cancel != nil {
		(*cancel)()
	}
	paletteScreen.Store(saved.Palette.Screen)
	if err := code.Open(saved, cacheDir); err != nil {
		return err
	}
	go reindex()
	return nil
}

// setLayout resizes the palette to a named layout (see layouts).
func setLayout(name string) {
	if _, ok := layouts[name]; !ok {
		return
	}
	layout.Store(name)
	screen, _ := palette.GetScreen()
	placeOn(screen)
}

// targetScreen is the screen the palette should open on, per settings.
func targetScreen() *application.Screen {
	where, _ := paletteScreen.Load().(string)
	var x, y int
	ok := false
	switch where {
	case "primary":
	case "mouse":
		x, y, ok = mousePoint()
	default:
		if x, y, ok = focusedWindowPoint(); !ok {
			x, y, ok = mousePoint()
		}
	}
	if ok {
		if s := app.Screen.ScreenNearestDipPoint(application.Point{X: x, Y: y}); s != nil {
			return s
		}
	}
	return app.Screen.GetPrimary()
}

// placeOn sizes the palette for the current layout on screen, centered
// horizontally with its top edge in the upper part of the screen, as
// Spotlight does.
func placeOn(screen *application.Screen) {
	name, _ := layout.Load().(string)
	size, ok := layouts[name]
	if !ok {
		size = layouts["compact"]
	}
	if screen == nil {
		palette.SetSize(size.width, size.height)
		palette.Center()
		return
	}
	area := screen.WorkArea
	x := area.X + (area.Width-size.width)/2
	y := area.Y + area.Height*18/100
	y = min(y, area.Y+area.Height-size.height-20)
	palette.SetBounds(application.Rect{X: x, Y: max(y, area.Y), Width: size.width, Height: size.height})
}

// reindex updates the index in the background, reporting progress to the UI.
// A request made while indexing runs another pass when the current one ends.
func reindex() {
	reindexPending.Store(true)
	if !indexing.CompareAndSwap(false, true) {
		return
	}
	for reindexPending.Swap(false) {
		ctx, cancel := context.WithCancel(context.Background())
		cancelReindex.Store(&cancel)
		err := code.Reindex(ctx, func(p code.Progress) {
			app.Event.Emit("index:progress", p)
		})
		cancel()
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("reindex: %v", err)
		}
		app.Event.Emit("index:done", code.Roots())
	}
	indexing.Store(false)
	if reindexPending.Load() {
		go reindex()
	}
}
