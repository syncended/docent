//go:build !darwin

package main

func mousePoint() (x, y int, ok bool) { return 0, 0, false }

func focusedWindowPoint() (x, y int, ok bool) { return 0, 0, false }
