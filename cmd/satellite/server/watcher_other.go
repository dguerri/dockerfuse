// Copyright 2026 the dockerfuse authors. All rights reserved.

//go:build !linux

package server

import (
	"errors"

	"github.com/dguerri/dockerfuse/pkg/rpccommon"
)

// fsWatcher is a non-functional stub on non-linux platforms so the
// satellite still cross-compiles. The satellite only ships on linux in
// production; this exists to keep developer builds on macOS/etc happy.
type fsWatcher struct{}

func newFSWatcher() *fsWatcher { return &fsWatcher{} }

func (w *fsWatcher) start(string) error { return errors.New("fs watcher: unsupported on this platform") }
func (w *fsWatcher) stop()               {}
func (w *fsWatcher) wait(uint32) ([]rpccommon.FsEvent, bool) {
	return nil, false
}
