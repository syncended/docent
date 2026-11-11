//go:build darwin

package code

/*
#cgo LDFLAGS: -framework CoreServices
#include <CoreServices/CoreServices.h>
#include <dispatch/dispatch.h>
#include <stdlib.h>

extern void docentFSEvents(uintptr_t id, size_t n, char **paths, FSEventStreamEventFlags *flags);

static void docentFSEventsCallback(ConstFSEventStreamRef s, void *info, size_t n, void *paths,
		const FSEventStreamEventFlags flags[], const FSEventStreamEventId ids[]) {
	docentFSEvents((uintptr_t)info, n, (char **)paths, (FSEventStreamEventFlags *)flags);
}

// docentStartStream watches the tree at path, delivering its events on a
// serial queue, so in order.
static FSEventStreamRef docentStartStream(uintptr_t id, const char *path) {
	static dispatch_queue_t queue;
	static dispatch_once_t once;
	dispatch_once(&once, ^{ queue = dispatch_queue_create("docent.fsevents", DISPATCH_QUEUE_SERIAL); });
	CFStringRef p = CFStringCreateWithCString(NULL, path, kCFStringEncodingUTF8);
	CFArrayRef paths = CFArrayCreate(NULL, (const void **)&p, 1, &kCFTypeArrayCallBacks);
	FSEventStreamContext ctx = {0, (void *)id, NULL, NULL, NULL};
	FSEventStreamRef s = FSEventStreamCreate(NULL, docentFSEventsCallback, &ctx, paths,
		kFSEventStreamEventIdSinceNow, 0.05,
		kFSEventStreamCreateFlagFileEvents | kFSEventStreamCreateFlagNoDefer | kFSEventStreamCreateFlagWatchRoot);
	CFRelease(paths);
	CFRelease(p);
	if (s == NULL) {
		return NULL;
	}
	FSEventStreamSetDispatchQueue(s, queue);
	if (!FSEventStreamStart(s)) {
		FSEventStreamInvalidate(s);
		FSEventStreamRelease(s);
		return NULL;
	}
	return s;
}

static void docentStopStream(FSEventStreamRef s) {
	FSEventStreamStop(s);
	FSEventStreamInvalidate(s);
	FSEventStreamRelease(s);
}
*/
import "C"

import (
	"path/filepath"
	"strings"
	"sync"
	"unsafe"
)

// On macOS each root is one FSEvents stream over its whole tree. kqueue,
// fsnotify's backend here, holds a descriptor for every file of a watched
// directory, which a large root runs out of. The watched directories are
// only bookkeeping: events outside them are dropped by note.

// stream is the FSEvents stream of a root.
type stream struct {
	ref C.FSEventStreamRef
	id  uintptr
	// root is the root's path; real is it with symlinks resolved, as
	// FSEvents reports paths.
	root, real string
	out        chan []event
	done       chan struct{}
}

const lostFlags = C.kFSEventStreamEventFlagMustScanSubDirs | C.kFSEventStreamEventFlagUserDropped |
	C.kFSEventStreamEventFlagKernelDropped | C.kFSEventStreamEventFlagRootChanged

var (
	// streamMu guards byID, which the FSEvents queue reads.
	streamMu   sync.Mutex
	byID       = map[uintptr]*stream{}
	nextStream uintptr

	// streams holds the running streams by root path, and events is where
	// they deliver; both guarded by watchMu.
	streams map[string]*stream
	events  chan []event
)

// startEvents prepares for streams. Called with watchMu held.
func startEvents() (<-chan []event, error) {
	streams = map[string]*stream{}
	events = make(chan []event, 64)
	return events, nil
}

// stopEvents stops every stream. Called with watchMu held.
func stopEvents() {
	for root, s := range streams {
		stopStream(s)
		delete(streams, root)
	}
	streams, events = nil, nil
}

// watchRoots runs a stream for each of paths and none for others. Called
// with watchMu held.
func watchRoots(paths []string) {
	want := map[string]bool{}
	for _, p := range paths {
		want[p] = true
	}
	for root, s := range streams {
		if !want[root] {
			stopStream(s)
			delete(streams, root)
		}
	}
	for _, p := range paths {
		if streams[p] != nil {
			continue
		}
		if s := startStream(p); s != nil {
			streams[p] = s
		}
	}
}

func startStream(root string) *stream {
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil
	}
	streamMu.Lock()
	nextStream++
	s := &stream{id: nextStream, root: root, real: real, out: events, done: make(chan struct{})}
	byID[s.id] = s
	streamMu.Unlock()
	cpath := C.CString(real)
	defer C.free(unsafe.Pointer(cpath))
	s.ref = C.docentStartStream(C.uintptr_t(s.id), cpath)
	if s.ref == nil {
		streamMu.Lock()
		delete(byID, s.id)
		streamMu.Unlock()
		return nil
	}
	return s
}

func stopStream(s *stream) {
	streamMu.Lock()
	delete(byID, s.id)
	streamMu.Unlock()
	close(s.done)
	C.docentStopStream(s.ref)
}

// addWatch has nothing to do: the stream covers the whole root.
func addWatch(string) bool { return true }

// removeWatch has nothing to do: the stream covers the whole root.
func removeWatch(string) {}

//export docentFSEvents
func docentFSEvents(id C.uintptr_t, n C.size_t, paths **C.char, flags *C.FSEventStreamEventFlags) {
	streamMu.Lock()
	s := byID[uintptr(id)]
	streamMu.Unlock()
	if s == nil {
		return
	}
	ps, fs := unsafe.Slice(paths, int(n)), unsafe.Slice(flags, int(n))
	batch := make([]event, 0, len(ps))
	for i, p := range ps {
		f := fs[i]
		if f&lostFlags != 0 {
			batch = append(batch, event{path: s.root, lost: true})
			continue
		}
		batch = append(batch, event{
			path:    s.fromReal(C.GoString(p)),
			dir:     f&C.kFSEventStreamEventFlagItemIsDir != 0,
			created: f&C.kFSEventStreamEventFlagItemCreated != 0,
			removed: f&(C.kFSEventStreamEventFlagItemRemoved|C.kFSEventStreamEventFlagItemRenamed) != 0,
		})
	}
	select {
	case s.out <- batch:
	case <-s.done:
	}
}

// fromReal maps a path FSEvents reported back under the root's own path.
func (s *stream) fromReal(p string) string {
	if p == s.real {
		return s.root
	}
	if rest, ok := strings.CutPrefix(p, s.real+"/"); ok {
		return filepath.Join(s.root, rest)
	}
	return p
}
