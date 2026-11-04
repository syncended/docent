//go:build !darwin

package main

// setRegularApp is macOS-only: elsewhere the overview window shows on the
// taskbar by itself.
func setRegularApp(bool) {}
