package main

import (
	"runtime"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/syncended/docent/core/code"
)

// The overview window is an ordinary application window with the project
// tree and files in tabs (DESIGN.md, Overview window). Closing it hides it,
// so it reopens with the same tabs until Docent restarts.

var (
	overview *application.WebviewWindow
	// overPalette is set while the palette is summoned over the overview
	// window: results then open as tabs there, and hiding the palette
	// returns focus to it.
	overPalette atomic.Bool
)

// overviewFile is a file to open as a tab in the overview window. Content,
// if set, is unsaved text moved from the palette viewer.
type overviewFile struct {
	Hit     code.Hit `json:"hit"`
	Content *string  `json:"content,omitempty"`
}

func init() {
	application.RegisterEvent[overviewFile]("overview:open")
	application.RegisterEvent[string]("overview:close")
}

func newOverview() {
	// Outside macOS the window is frameless and transparent: the frontend
	// draws the backdrop with corners matching its bubbles (.overview.frameless).
	frameless := runtime.GOOS != "darwin"
	background := application.BackgroundTypeSolid
	if frameless {
		background = application.BackgroundTypeTransparent
	}
	overview = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   "overview",
		Title:  "Docent",
		Width:  1280,
		Height: 820,
		Hidden: true,
		URL:    "/#overview",
		// The frontend's backdrop colour (.overview), shown before it loads.
		BackgroundColour: application.NewRGB(0x0e, 0x0e, 0x11),
		BackgroundType:   background,
		// The frontend's top bar is the title bar. On macOS the traffic
		// lights sit over it (frontend .ov-top.mac); elsewhere the window is
		// frameless and the frontend draws the window buttons.
		Frameless: frameless,
		Mac: application.MacWindow{
			TitleBar:   application.MacTitleBarHiddenInset,
			Appearance: application.NSAppearanceNameDarkAqua,
		},
		Windows: application.WindowsWindow{
			Theme: application.Dark,
			// No system rounding or shadow: they would not match the drawn
			// corners.
			DisableFramelessWindowDecorations: true,
		},
	})
	// The close button asks the frontend first: it hides the window, or
	// asks about unsaved edits (see hideOverview).
	overview.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		app.Event.Emit("overview:close", "")
	})
}

// showOverview opens the overview window, or brings it forward.
func showOverview() {
	setRegularApp(true)
	overview.Show()
	if overview.IsMinimised() {
		overview.UnMinimise()
	}
	overview.Focus()
}

func hideOverview() {
	overview.Hide()
	setRegularApp(false)
}

// openInOverview opens f as a tab in the overview window and hides the palette.
func openInOverview(f overviewFile) {
	overPalette.Store(false)
	hidePalette()
	showOverview()
	app.Event.Emit("overview:open", f)
}

// overviewFocused reports whether the overview window is the one the user
// is working in, so that the palette summoned now searches into it.
func overviewFocused() bool {
	return overview.IsVisible() && !overview.IsMinimised() && overview.IsFocused()
}
