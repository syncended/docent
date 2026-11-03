//go:build !darwin && !windows

package main

// paletteTop is where the palette's top edge sits, in percent of the
// screen height.
const paletteTop = 18

func mousePoint() (x, y int, ok bool) { return 0, 0, false }

func focusedWindowPoint() (x, y int, ok bool) { return 0, 0, false }
