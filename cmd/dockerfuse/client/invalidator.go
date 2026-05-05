// Copyright 2026 the dockerfuse authors. All rights reserved.

package client

import (
	"context"
	"errors"
	"log/slog"
	"path"
	"strings"
	"time"

	fusefs "github.com/hanwen/go-fuse/v2/fs"

	"github.com/dguerri/dockerfuse/pkg/rpccommon"
)

// waitForEventsTimeoutMs is the long-poll deadline the host sends to the
// satellite. Long enough to amortize wakeups, short enough that a hung
// satellite is noticed promptly.
const waitForEventsTimeoutMs uint32 = 30_000

// Watch tells the satellite to start a recursive filesystem watch rooted at
// path. Idempotent.
func (d *DockerFuseClient) Watch(_ context.Context, root string) error {
	if d.rpcClient == nil {
		return errors.New("rpc client not connected")
	}
	var reply rpccommon.WatchReply
	return d.rpcClient.Call("DockerFuseFSOps.Watch", rpccommon.WatchRequest{Root: root}, &reply)
}

// WaitForEvents long-polls the satellite for buffered FsEvents.
func (d *DockerFuseClient) WaitForEvents(_ context.Context, timeoutMs uint32) ([]rpccommon.FsEvent, bool, error) {
	if d.rpcClient == nil {
		return nil, false, errors.New("rpc client not connected")
	}
	var reply rpccommon.WaitForEventsReply
	if err := d.rpcClient.Call("DockerFuseFSOps.WaitForEvents",
		rpccommon.WaitForEventsRequest{TimeoutMs: timeoutMs}, &reply); err != nil {
		return nil, false, err
	}
	return reply.Events, reply.Overflowed, nil
}

// invalidatorClient is the subset of DockerFuseClient that the invalidator
// needs. Defined here so tests can substitute a fake.
type invalidatorClient interface {
	Watch(ctx context.Context, root string) error
	WaitForEvents(ctx context.Context, timeoutMs uint32) ([]rpccommon.FsEvent, bool, error)
}

// invalidator drives FUSE_NOTIFY_INVAL_ENTRY on the host's go-fuse Server
// in response to filesystem events from the satellite. Closes the gap a
// pure pull-based FUSE mount has when files inside the served tree are
// mutated by processes other than the FUSE client itself.
type invalidator struct {
	client invalidatorClient
	root   *fusefs.Inode
	stop   chan struct{}
}

// StartInvalidator starts a goroutine that long-polls the satellite for
// filesystem events and fires (*Inode).NotifyEntry on the parent inode of
// each affected entry. root is the host-side FUSE root inode (returned by
// the rootEmbedder.EmbeddedInode() after fs.Mount). watchRoot is the
// satellite-side path to watch.
//
// The returned cancel function stops the goroutine and waits for it to exit.
func StartInvalidator(c *DockerFuseClient, root *fusefs.Inode, watchRoot string) (func(), error) {
	if root == nil {
		return nil, errors.New("invalidator: nil root inode")
	}
	if err := c.Watch(context.Background(), watchRoot); err != nil {
		return nil, err
	}
	inv := &invalidator{
		client: c,
		root:   root,
		stop:   make(chan struct{}),
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		inv.run()
	}()
	return func() {
		close(inv.stop)
		<-done
	}, nil
}

func (inv *invalidator) run() {
	for {
		select {
		case <-inv.stop:
			return
		default:
		}
		events, overflowed, err := inv.client.WaitForEvents(context.Background(), waitForEventsTimeoutMs)
		if err != nil {
			slog.Debug("invalidator: WaitForEvents failed", "error", err)
			// Back off briefly so a broken connection doesn't busy-spin.
			select {
			case <-inv.stop:
				return
			case <-time.After(time.Second):
			}
			continue
		}
		if overflowed {
			slog.Warn("invalidator: satellite event buffer overflowed; cache may be stale")
		}
		for _, ev := range events {
			parent := resolve(inv.root, ev.ParentPath)
			if parent == nil {
				// Kernel never looked this directory up — nothing cached
				// to invalidate.
				continue
			}
			if ev.Name == "" {
				continue
			}
			st := parent.NotifyEntry(ev.Name)
			slog.Debug("invalidator: NotifyEntry",
				"parent", ev.ParentPath, "name", ev.Name, "op", ev.Op, "status", st)
		}
	}
}

// resolve walks the host's FUSE node tree from root following the
// slash-separated parentPath emitted by the satellite. Returns nil if any
// component along the path has not been looked up yet (i.e., the kernel
// has no cache entry to invalidate).
func resolve(root *fusefs.Inode, parentPath string) *fusefs.Inode {
	parentPath = path.Clean(parentPath)
	if parentPath == "." || parentPath == "/" || parentPath == "" {
		return root
	}
	cur := root
	for _, part := range strings.Split(strings.Trim(parentPath, "/"), "/") {
		if part == "" {
			continue
		}
		cur = cur.GetChild(part)
		if cur == nil {
			return nil
		}
	}
	return cur
}
