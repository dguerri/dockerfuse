// Copyright 2026 the dockerfuse authors. All rights reserved.

//go:build linux

package server

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/dguerri/dockerfuse/pkg/rpccommon"
)

// eventBufferSize bounds the in-memory queue between the inotify reader
// goroutine and WaitForEvents callers. Overflow is reported with the
// Overflowed flag rather than blocking the reader, so a slow host cannot
// cause the satellite to back up.
const eventBufferSize = 8192

// inotifyMask is the set of events we react to. We deliberately omit
// IN_ACCESS / IN_OPEN / IN_CLOSE_NOWRITE because they do not affect the
// host's view of the directory or its metadata.
const inotifyMask = unix.IN_CREATE | unix.IN_DELETE |
	unix.IN_MOVED_FROM | unix.IN_MOVED_TO |
	unix.IN_MODIFY | unix.IN_ATTRIB |
	unix.IN_DELETE_SELF | unix.IN_MOVE_SELF

// fsWatcher owns an inotify fd and a recursive set of watch descriptors
// rooted at root. It serializes events into a bounded channel that
// WaitForEvents drains.
type fsWatcher struct {
	mu       sync.Mutex
	fd       int
	root     string
	wdToDir  map[int32]string
	dirToWd  map[string]int32
	events   chan rpccommon.FsEvent
	stopCh   chan struct{}
	stopOnce sync.Once

	// overflowed is set when an event had to be dropped because the
	// channel was full; it is cleared on the next WaitForEvents reply.
	overflowMu sync.Mutex
	overflowed bool
}

func newFSWatcher() *fsWatcher {
	return &fsWatcher{
		fd:      -1,
		wdToDir: make(map[int32]string),
		dirToWd: make(map[string]int32),
	}
}

// start initializes inotify, walks the tree under root adding watches to
// every directory, and spawns the reader goroutine. It is idempotent for
// the same root and replaces the previous watch otherwise.
func (w *fsWatcher) start(root string) error {
	w.mu.Lock()
	if w.fd >= 0 && w.root == root {
		w.mu.Unlock()
		return nil
	}
	if w.fd >= 0 {
		w.mu.Unlock()
		w.stop()
		w.mu.Lock()
	}

	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC)
	if err != nil {
		w.mu.Unlock()
		return err
	}
	w.fd = fd
	w.root = root
	w.wdToDir = make(map[int32]string)
	w.dirToWd = make(map[string]int32)
	w.events = make(chan rpccommon.FsEvent, eventBufferSize)
	w.stopCh = make(chan struct{})
	w.stopOnce = sync.Once{}
	w.mu.Unlock()

	if err := w.addRecursive(root); err != nil {
		w.stop()
		return err
	}

	go w.readLoop()
	return nil
}

// stop releases the inotify fd and signals the reader goroutine to exit.
// Safe to call multiple times.
func (w *fsWatcher) stop() {
	w.stopOnce.Do(func() {
		w.mu.Lock()
		fd := w.fd
		w.fd = -1
		stopCh := w.stopCh
		w.mu.Unlock()
		if fd >= 0 {
			// Closing the fd unblocks any in-flight Read in readLoop.
			_ = unix.Close(fd)
		}
		if stopCh != nil {
			close(stopCh)
		}
	})
}

// wait blocks up to timeoutMs for at least one event, then drains and
// returns everything currently buffered.
func (w *fsWatcher) wait(timeoutMs uint32) ([]rpccommon.FsEvent, bool) {
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	select {
	case ev, ok := <-w.events:
		if !ok {
			return nil, w.takeOverflow()
		}
		out := []rpccommon.FsEvent{ev}
		for {
			select {
			case ev, ok := <-w.events:
				if !ok {
					return out, w.takeOverflow()
				}
				out = append(out, ev)
			default:
				return out, w.takeOverflow()
			}
		}
	case <-time.After(time.Until(deadline)):
		return nil, w.takeOverflow()
	}
}

func (w *fsWatcher) takeOverflow() bool {
	w.overflowMu.Lock()
	defer w.overflowMu.Unlock()
	o := w.overflowed
	w.overflowed = false
	return o
}

func (w *fsWatcher) markOverflow() {
	w.overflowMu.Lock()
	w.overflowed = true
	w.overflowMu.Unlock()
}

// addRecursive walks the tree rooted at path and adds an inotify watch on
// every directory. Errors on individual entries (e.g. permission denied)
// are logged but not fatal.
func (w *fsWatcher) addRecursive(path string) error {
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("fs watcher: walk %s: %v", p, err)
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		if err := w.addOne(p); err != nil {
			log.Printf("fs watcher: add %s: %v", p, err)
		}
		return nil
	})
}

// addOne registers a single directory with inotify.
// Caller must ensure path is a directory.
func (w *fsWatcher) addOne(path string) error {
	w.mu.Lock()
	fd := w.fd
	w.mu.Unlock()
	if fd < 0 {
		return errors.New("watcher stopped")
	}
	wd, err := unix.InotifyAddWatch(fd, path, inotifyMask)
	if err != nil {
		return err
	}
	w.mu.Lock()
	w.wdToDir[int32(wd)] = path
	w.dirToWd[path] = int32(wd)
	w.mu.Unlock()
	return nil
}

func (w *fsWatcher) removeWd(wd int32) {
	w.mu.Lock()
	p, ok := w.wdToDir[wd]
	if ok {
		delete(w.wdToDir, wd)
		delete(w.dirToWd, p)
	}
	w.mu.Unlock()
}

// readLoop consumes raw inotify events from the kernel, translates them
// into FsEvents tagged with parent path + child name, and pushes them on
// the channel. Exits when the inotify fd is closed (stop()).
func (w *fsWatcher) readLoop() {
	buf := make([]byte, 64*1024)
	for {
		w.mu.Lock()
		fd := w.fd
		w.mu.Unlock()
		if fd < 0 {
			return
		}
		n, err := unix.Read(fd, buf)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			// Closed fd: exit cleanly.
			return
		}
		w.parseAndDispatch(buf[:n])
	}
}

// parseAndDispatch walks one inotify Read buffer, emits an FsEvent per
// raw event, and adds new watches when a directory is created.
func (w *fsWatcher) parseAndDispatch(buf []byte) {
	const sizeofEvent = int(unsafe.Sizeof(unix.InotifyEvent{}))
	off := 0
	for off+sizeofEvent <= len(buf) {
		raw := (*unix.InotifyEvent)(unsafe.Pointer(&buf[off]))
		nameLen := int(raw.Len)
		name := ""
		if nameLen > 0 {
			start := off + sizeofEvent
			end := start + nameLen
			if end > len(buf) {
				break
			}
			// Strip trailing NULs (inotify pads names).
			b := buf[start:end]
			for i, c := range b {
				if c == 0 {
					b = b[:i]
					break
				}
			}
			name = string(b)
		}
		w.handleRaw(raw.Wd, raw.Mask, name)
		off += sizeofEvent + nameLen
	}
}

func (w *fsWatcher) handleRaw(wd int32, mask uint32, name string) {
	w.mu.Lock()
	parent, ok := w.wdToDir[wd]
	root := w.root
	w.mu.Unlock()
	if !ok {
		// Watch already torn down — IN_IGNORED on a stale wd, ignore.
		return
	}

	// On dir teardown the kernel auto-removes the watch; mirror in our
	// maps so we don't leak.
	if mask&unix.IN_IGNORED != 0 {
		w.removeWd(wd)
		return
	}

	op := opFromMask(mask)
	if op == 0 {
		return
	}

	// IN_CREATE on a directory means a new subdir to recursively watch.
	if mask&unix.IN_CREATE != 0 && mask&unix.IN_ISDIR != 0 && name != "" {
		full := filepath.Join(parent, name)
		if info, err := os.Lstat(full); err == nil && info.IsDir() {
			if err := w.addOne(full); err != nil {
				log.Printf("fs watcher: add new subdir %s: %v", full, err)
			}
			// Catch entries created between mkdir and our addOne.
			if subErr := w.addRecursive(full); subErr != nil {
				log.Printf("fs watcher: recurse %s: %v", full, subErr)
			}
		}
	}

	rel, err := filepath.Rel(root, parent)
	if err != nil {
		// Should not happen — parent is always under root.
		return
	}
	ev := rpccommon.FsEvent{
		ParentPath: filepath.ToSlash(rel),
		Name:       name,
		Op:         op,
	}
	select {
	case w.events <- ev:
	default:
		w.markOverflow()
	}
}

// opFromMask collapses the inotify bitmask into one of the four FsEvent op
// codes. Returns 0 for events the host doesn't care about.
func opFromMask(mask uint32) uint8 {
	switch {
	case mask&unix.IN_CREATE != 0:
		return rpccommon.FsEventCreated
	case mask&unix.IN_DELETE != 0, mask&unix.IN_DELETE_SELF != 0:
		return rpccommon.FsEventDeleted
	case mask&unix.IN_MOVED_FROM != 0, mask&unix.IN_MOVED_TO != 0, mask&unix.IN_MOVE_SELF != 0:
		return rpccommon.FsEventMoved
	case mask&unix.IN_MODIFY != 0, mask&unix.IN_ATTRIB != 0:
		return rpccommon.FsEventModified
	}
	return 0
}
