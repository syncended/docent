//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

// Global coordinates with the origin at the top left of the primary screen.

static void mousePoint(double *x, double *y) {
	NSPoint p = [NSEvent mouseLocation];
	CGFloat height = NSScreen.screens.firstObject.frame.size.height;
	*x = p.x;
	*y = height - p.y;
}

// focusedWindowCenter finds the frontmost normal window of the active app.
// Window bounds are available without screen recording permission.
static int focusedWindowCenter(double *x, double *y) {
	pid_t pid = NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier;
	CFArrayRef windows = CGWindowListCopyWindowInfo(
		kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
	int found = 0;
	for (CFIndex i = 0; windows && i < CFArrayGetCount(windows) && !found; i++) {
		NSDictionary *w = (__bridge NSDictionary *)CFArrayGetValueAtIndex(windows, i);
		if ([w[(id)kCGWindowOwnerPID] intValue] != pid || [w[(id)kCGWindowLayer] intValue] != 0) {
			continue;
		}
		CGRect r;
		if (CGRectMakeWithDictionaryRepresentation((__bridge CFDictionaryRef)w[(id)kCGWindowBounds], &r)) {
			*x = CGRectGetMidX(r);
			*y = CGRectGetMidY(r);
			found = 1;
		}
	}
	if (windows) CFRelease(windows);
	return found;
}
*/
import "C"

// mousePoint returns the mouse pointer position.
func mousePoint() (x, y int, ok bool) {
	var cx, cy C.double
	C.mousePoint(&cx, &cy)
	return int(cx), int(cy), true
}

// focusedWindowPoint returns the center of the focused window of the
// active application.
func focusedWindowPoint() (x, y int, ok bool) {
	var cx, cy C.double
	if C.focusedWindowCenter(&cx, &cy) == 0 {
		return 0, 0, false
	}
	return int(cx), int(cy), true
}
