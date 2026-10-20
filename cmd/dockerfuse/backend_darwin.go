//go:build darwin

package main

import (
	"flag"
	"fmt"
	"log/slog"

	"github.com/dguerri/dockerfuse/cmd/dockerfuse/client"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fskit"
)

var backend string

func init() {
	flag.StringVar(&backend, "backend", "auto", "FUSE backend: 'auto' (default, uses FSKit if available and falls back to kext), 'fskit', or 'kext' ('macfuse')")
}

func mountFS(mountPoint string, newRoot func() *client.Node, opts *fs.Options) (Server, error) {
	switch backend {
	case "", "auto":
		if fskit.Available() {
			slog.Debug("FSKit framework detected, attempting FSKit mount")
			root := newRoot()
			server, err := fskit.MountFS(mountPoint, root, opts)
			if err == nil {
				slog.Info("mounted using FSKit backend")
				return server, nil
			}
			slog.Warn("FSKit mount failed, falling back to kernel extension backend", "error", err)
		} else {
			slog.Debug("FSKit framework not found, using kernel extension backend")
		}
		root := newRoot()
		server, err := fs.Mount(mountPoint, root, opts)
		return server, err
	case "fskit":
		root := newRoot()
		server, err := fskit.MountFS(mountPoint, root, opts)
		return server, err
	case "kext", "macfuse":
		root := newRoot()
		server, err := fs.Mount(mountPoint, root, opts)
		return server, err
	default:
		return nil, fmt.Errorf("unsupported backend %q (valid values: 'auto', 'fskit', 'kext', 'macfuse')", backend)
	}
}
