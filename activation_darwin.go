//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

static void setRegular(int regular) {
	if (regular) {
		[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
		[NSApp activateIgnoringOtherApps:YES];
	} else {
		[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
	}
}
*/
import "C"

import "github.com/wailsapp/wails/v3/pkg/application"

// setRegularApp switches between an accessory app, so the palette never
// activates Docent, and a regular one with a Dock icon while the overview
// window is open (DESIGN.md, Technical decisions).
func setRegularApp(regular bool) {
	v := C.int(0)
	if regular {
		v = 1
	}
	application.InvokeSync(func() { C.setRegular(v) })
}
