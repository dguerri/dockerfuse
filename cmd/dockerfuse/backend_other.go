//go:build !darwin

package main

import (
	"github.com/dguerri/dockerfuse/cmd/dockerfuse/client"
	"github.com/hanwen/go-fuse/v2/fs"
)

func mountFS(mountPoint string, newRoot func() *client.Node, opts *fs.Options) (Server, error) {
	root := newRoot()
	server, err := fs.Mount(mountPoint, root, opts)
	return server, err
}
