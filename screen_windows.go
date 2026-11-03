//go:build windows

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

// paletteTop is where the palette's top edge sits, in percent of the
// screen height.
const paletteTop = 25

// Windows reports physical pixels; the palette is placed in DIPs.

// mousePoint returns the mouse pointer position.
func mousePoint() (x, y int, ok bool) {
	px, py, ok := w32.GetCursorPos()
	if !ok {
		return 0, 0, false
	}
	p := app.Screen.PhysicalToDipPoint(application.Point{X: px, Y: py})
	return p.X, p.Y, true
}

// focusedWindowPoint returns the center of the foreground window.
func focusedWindowPoint() (x, y int, ok bool) {
	hwnd := w32.GetForegroundWindow()
	if hwnd == 0 {
		return 0, 0, false
	}
	r := w32.GetWindowRect(hwnd)
	// Minimized windows are parked at -32000.
	if r.Right <= r.Left || r.Left <= -32000 {
		return 0, 0, false
	}
	p := app.Screen.PhysicalToDipPoint(application.Point{X: int(r.Left+r.Right) / 2, Y: int(r.Top+r.Bottom) / 2})
	return p.X, p.Y, true
}
