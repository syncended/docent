// Command docent is the desktop shell: a background app with a global hotkey
// that shows the search palette over the active window.
package main

import (
	"context"
	"embed"
	"errors"
	"log"
	"strings"
	"sync/atomic"
	"time"

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

	// placed is where placeOn last put the palette; a different position
	// means the user dragged it.
	placed atomic.Pointer[application.Rect]
	// moved is where the user dragged the palette to, kept until restart.
	moved atomic.Pointer[anchor]
	// pending is the screen the palette is shown on once the frontend has
	// painted (see showSettings); nil when it is in place.
	pending atomic.Pointer[application.Screen]
)

// anchor is the palette's horizontal center and top edge, as fractions of
// a screen's work area, so it applies to any screen.
type anchor struct{ x, y float64 }

// layouts are the palette window sizes: a Spotlight-like search bar, the bar
// with results below it, and the file viewer with or without the side column.
var layouts = map[string]struct{ width, height int }{
	"compact": {720, 92},
	"results": {720, 600},
	"viewer":  {1100, 740},
	// The viewer with the side column (frontend .side, 240px + 8px gap).
	"viewer-side": {1348, 740},
}

func init() {
	application.RegisterEvent[code.Progress]("index:progress")
	application.RegisterEvent[[]code.RootInfo]("index:done")
	application.RegisterEvent[string]("palette:shown")
	application.RegisterEvent[string]("palette:search")
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
	current.Store(cfg)
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
		// A second launch brings the running Docent's overview window
		// forward (DESIGN.md, Overview window).
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "dev.syncended.docent",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) { showOverview() },
		},
	})

	// A non-activating floating panel, like Spotlight: it takes keyboard
	// input without activating Docent, so focus stays with the previous app.
	palette = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "palette",
		Width:     layouts["compact"].width,
		Height:    layouts["compact"].height,
		Frameless: true,
		// The size follows the layout (see layouts); without this Windows
		// adds resize borders to the frameless window.
		DisableResize: true,
		Hidden:        true,
		URL:           "/",
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

	newOverview()
	// Launching Docent opens the overview window; clicking its Dock icon
	// brings it back.
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) { showOverview() })
	app.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(*application.ApplicationEvent) { showOverview() })

	tray := app.SystemTray.New()
	tray.SetLabel("⌕")
	tray.SetTooltip("Docent")
	menu := app.NewMenu()
	menu.Add("Open Docent").OnClick(func(*application.Context) { showOverview() })
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

// togglePalette handles the hotkey: it summons Docent, goes from the viewer
// to the palette, and hides the palette.
func togglePalette() {
	if !palette.IsVisible() {
		showPalette()
		return
	}
	if name, _ := layout.Load().(string); strings.HasPrefix(name, "viewer") {
		app.Event.Emit("palette:search", "")
		return
	}
	hidePalette()
}

// showPalette summons the palette.
func showPalette() { summon("palette:shown") }

// summon shows the palette and sends event to the frontend. Over the focused
// overview window its results open there; the event's data tells which.
func summon(event string) {
	over := overviewFocused()
	overPalette.Store(over)
	target := ""
	if over {
		target = "overview"
	}
	showOffScreen(event, target)
}

// showSettings shows the palette on the settings screen, at the settings
// size (the frontend's "results" layout).
func showSettings() {
	overPalette.Store(overviewFocused())
	layout.Store("results")
	showOffScreen("settings:open", "")
}

// showOffScreen shows the palette and emits event to the frontend. A hidden
// webview does not paint, so shown in place the palette would first flash
// the last frame it painted before it was hidden. It is shown off screen
// and moved into place once the frontend has painted (presentPending), or
// after a timeout.
func showOffScreen(event, data string) {
	screen := targetScreen()
	if screen == nil {
		screen = app.Screen.GetPrimary()
	}
	pending.Store(screen)
	app.Event.Emit(event, data)
	name, _ := layout.Load().(string)
	size, ok := layouts[name]
	if !ok {
		size = layouts["compact"]
	}
	palette.SetBounds(application.Rect{X: -20000, Y: -20000, Width: size.width, Height: size.height})
	palette.Show()
	time.AfterFunc(400*time.Millisecond, presentPending)
}

// presentPending moves the palette shown off screen into place.
func presentPending() {
	if screen := pending.Swap(nil); screen != nil {
		placeOn(screen)
		palette.Focus()
	}
}

func hidePalette() {
	rememberMove()
	palette.Hide()
	if overPalette.Swap(false) && overview.IsVisible() {
		overview.Focus()
	}
}

// pickFolder asks for a directory with the system dialog, attached to the
// window w (the palette or the overview window). It returns "" if the user
// cancels.
func pickFolder(w *application.WebviewWindow) (string, error) {
	dialogOpen.Store(true)
	defer func() {
		dialogOpen.Store(false)
		w.Focus()
	}()
	return app.Dialog.OpenFile().
		SetTitle("Choose a folder with code").
		CanChooseFiles(false).
		CanChooseDirectories(true).
		CanCreateDirectories(false).
		AttachToWindow(w).
		PromptForSingleSelection()
}

// applyConfig saves cfg, reopens the index with the active project's roots
// and re-indexes in the background. A change of the active project is sent
// to both windows. Callers that loaded cfg to change it hold configMu.
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
	prev := current.Load()
	switched := prev == nil || prev.Active != saved.Active
	if switched {
		// A running agent must not search the next project for a question
		// about this one.
		stopAgent()
	}
	if err := code.Open(saved, cacheDir); err != nil {
		return err
	}
	current.Store(saved)
	if switched {
		from := ""
		if prev != nil {
			from = prev.Active
		}
		app.Event.Emit("project:switched", projectSwitch{From: from, To: saved.Active})
	}
	go reindex()
	return nil
}

// setLayout resizes the palette to a named layout (see layouts).
func setLayout(name string) {
	if _, ok := layouts[name]; !ok {
		return
	}
	rememberMove()
	layout.Store(name)
	// Off screen until painted: presentPending places it.
	if pending.Load() != nil {
		return
	}
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

// placeOn sizes the palette for the current layout on screen. By default it
// is centered horizontally with its top edge paletteTop percent down the
// screen, as Spotlight does; once dragged, it keeps the dragged position.
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
	size.width = min(size.width, area.Width)
	size.height = min(size.height, area.Height-20)
	x := area.X + (area.Width-size.width)/2
	y := area.Y + area.Height*paletteTop/100
	if a := moved.Load(); a != nil {
		x = area.X + int(a.x*float64(area.Width)) - size.width/2
		y = area.Y + int(a.y*float64(area.Height))
	}
	x = max(min(x, area.X+area.Width-size.width), area.X)
	y = max(min(y, area.Y+area.Height-size.height-20), area.Y)
	r := application.Rect{X: x, Y: y, Width: size.width, Height: size.height}
	placed.Store(&r)
	palette.SetBounds(r)
}

// rememberMove records the palette's position if the user dragged it away
// from where placeOn put it.
func rememberMove() {
	last := placed.Load()
	if last == nil || !palette.IsVisible() || pending.Load() != nil {
		return
	}
	b := palette.Bounds()
	// DIP rounding on scaled screens can shift the window by a pixel.
	if abs(b.X-last.X) <= 2 && abs(b.Y-last.Y) <= 2 {
		return
	}
	screen, _ := palette.GetScreen()
	if screen == nil || screen.WorkArea.Width == 0 || screen.WorkArea.Height == 0 {
		return
	}
	area := screen.WorkArea
	moved.Store(&anchor{
		x: float64(b.X+b.Width/2-area.X) / float64(area.Width),
		y: float64(b.Y-area.Y) / float64(area.Height),
	})
	placed.Store(&b)
}

func abs(n int) int { return max(n, -n) }

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
