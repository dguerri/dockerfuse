package client

import (
	"context"
	"io/fs"
	"log/slog"
	"path/filepath"
	"syscall"

	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

var _ = (fusefs.NodeCreater)((*Node)(nil))
var _ = (fusefs.NodeFlusher)((*Node)(nil))
var _ = (fusefs.NodeFsyncer)((*Node)(nil))
var _ = (fusefs.NodeGetattrer)((*Node)(nil))
var _ = (fusefs.NodeLinker)((*Node)(nil))
var _ = (fusefs.NodeLookuper)((*Node)(nil))
var _ = (fusefs.NodeLseeker)((*Node)(nil))
var _ = (fusefs.NodeMkdirer)((*Node)(nil))
var _ = (fusefs.NodeOpener)((*Node)(nil))
var _ = (fusefs.NodeReaddirer)((*Node)(nil))
var _ = (fusefs.NodeReader)((*Node)(nil))
var _ = (fusefs.NodeReadlinker)((*Node)(nil))
var _ = (fusefs.NodeReleaser)((*Node)(nil))
var _ = (fusefs.NodeRenamer)((*Node)(nil))
var _ = (fusefs.NodeRmdirer)((*Node)(nil))
var _ = (fusefs.NodeSetattrer)((*Node)(nil))
var _ = (fusefs.NodeSymlinker)((*Node)(nil))
var _ = (fusefs.NodeUnlinker)((*Node)(nil))
var _ = (fusefs.NodeWriter)((*Node)(nil))

// Node is a filesystem node used by dockerfuse to represent directories, files or links
type Node struct {
	fusefs.Inode

	Data             []byte
	fullPath         string
	fuseDockerClient DockerFuseClientInterface
}

// NewNode creates a new Node
func NewNode(fuseDockerClient DockerFuseClientInterface, fullPath string, linkTarget string) *Node {
	return &Node{
		Data:             []byte(linkTarget),
		fullPath:         fullPath,
		fuseDockerClient: fuseDockerClient,
	}
}

func (node *Node) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (newNode *fusefs.Inode, fh fusefs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	slog.Debug("Create() called", "path", node.fullPath, "name", name, "flags", flags, "mode", mode)

	fullPath := filepath.Clean(filepath.Join(node.fullPath, name))
	var fuseAttr statAttr
	fh, errno = node.fuseDockerClient.create(ctx, fullPath, int(flags), fs.FileMode(mode), &fuseAttr)
	if errno != 0 {
		slog.Error("remote error in create()", "path", fullPath, "errno", errno)
		return nil, nil, 0, errno
	}
	fuseFlags = flags
	out.Attr = fuseAttr.FuseAttr
	newNode = node.NewPersistentInode(ctx, NewNode(node.fuseDockerClient, fullPath, ""), fusefs.StableAttr{Ino: out.Ino})
	return
}

func (node *Node) Flush(ctx context.Context, fh fusefs.FileHandle) (syserr syscall.Errno) {
	slog.Debug("Flush() called", "path", node.fullPath, "fh", fh)

	return node.fuseDockerClient.close(ctx, fh)
}

func (node *Node) Release(ctx context.Context, fh fusefs.FileHandle) (syserr syscall.Errno) {
	slog.Debug("Release() called", "path", node.fullPath, "fh", fh)

	return node.fuseDockerClient.close(ctx, fh)
}

func (node *Node) Fsync(ctx context.Context, fh fusefs.FileHandle, flags uint32) (syserr syscall.Errno) {
	slog.Debug("Fsync() called", "path", node.fullPath, "fh", fh)

	return node.fuseDockerClient.fsync(ctx, fh, flags)
}

func (node *Node) Getattr(ctx context.Context, fh fusefs.FileHandle, out *fuse.AttrOut) (errno syscall.Errno) {
	slog.Debug("Getattr() called", "path", node.fullPath, "fh", fh)

	var fuseAttr statAttr
	errno = node.fuseDockerClient.stat(ctx, node.fullPath, &fuseAttr)
	if errno != 0 {
		// This is pretty noisy when targeting non-existing files
		slog.Debug("remote error in stat()", "path", node.fullPath, "errno", errno)
		return
	}

	out.Attr = fuseAttr.FuseAttr
	return
}

func (node *Node) Link(ctx context.Context, target fusefs.InodeEmbedder, name string, out *fuse.EntryOut) (newNode *fusefs.Inode, errno syscall.Errno) {
	slog.Debug("Link() called", "path", node.fullPath, "target", target, "name", name)

	newFullPath := filepath.Clean(filepath.Join(node.fullPath, name))
	errno = node.fuseDockerClient.link(ctx, target.(*Node).fullPath, newFullPath)
	if errno != 0 {
		slog.Error("remote error in link()", "path", node.fullPath, "errno", errno)
		return nil, errno
	}

	newNode = node.NewPersistentInode(ctx, NewNode(node.fuseDockerClient, newFullPath, ""), fusefs.StableAttr{Mode: target.EmbeddedInode().Mode()})
	return
}

func (node *Node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (n *fusefs.Inode, syserr syscall.Errno) {
	slog.Debug("Lookup() called", "path", node.fullPath, "name", name)
	fullPath := filepath.Clean(filepath.Join(node.fullPath, name))

	var fuseAttr statAttr
	syserr = node.fuseDockerClient.stat(ctx, fullPath, &fuseAttr)
	if syserr != 0 {
		// This is pretty noisy (e.g., for shell with auto completion)
		slog.Debug("remote error in stat()", "path", fullPath, "syserr", syserr)
		return nil, syserr
	}

	out.Attr = fuseAttr.FuseAttr

	stableAttr := fusefs.StableAttr{}
	switch {
	case out.Attr.Mode&fuse.S_IFDIR == fuse.S_IFDIR:
		slog.Debug("adding dir", "path", fullPath)
		stableAttr.Mode = fuse.S_IFDIR
	case out.Attr.Mode&fuse.S_IFLNK == fuse.S_IFLNK:
		slog.Debug("adding symlink", "path", fullPath)
		stableAttr.Mode = fuse.S_IFLNK
	case out.Attr.Mode&fuse.S_IFIFO == fuse.S_IFIFO:
		slog.Debug("adding FIFO", "path", fullPath)
		stableAttr.Mode = fuse.S_IFIFO
	default:
		slog.Debug("adding reg", "path", fullPath)
		stableAttr.Mode = fuse.S_IFREG
	}

	return node.NewPersistentInode(ctx, NewNode(node.fuseDockerClient, fullPath, fuseAttr.LinkTarget), stableAttr), 0
}

func (node *Node) Lseek(ctx context.Context, fh fusefs.FileHandle, off uint64, whence uint32) (n uint64, syserr syscall.Errno) {
	slog.Debug("Lseek() (Node) called", "path", node.fullPath)

	ntmp, syserr := node.fuseDockerClient.seek(ctx, fh, int64(off), int(whence))
	return uint64(ntmp), syserr
}

func (node *Node) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (newNode *fusefs.Inode, errno syscall.Errno) {
	slog.Debug("Mkdir() called", "path", node.fullPath, "name", name, "node", node)

	fullPath := filepath.Clean(filepath.Join(node.fullPath, name))
	var fuseAttr statAttr
	errno = node.fuseDockerClient.mkdir(ctx, fullPath, fs.FileMode(mode), &fuseAttr)
	if errno != 0 {
		slog.Error("remote error in mkdir()", "path", fullPath, "errno", errno)
		return nil, errno
	}
	out.Attr = fuseAttr.FuseAttr
	newNode = node.NewPersistentInode(ctx, NewNode(node.fuseDockerClient, fullPath, ""), fusefs.StableAttr{Mode: fuse.S_IFDIR, Ino: out.Ino})
	return
}

func (node *Node) Open(ctx context.Context, flags uint32) (fh fusefs.FileHandle, mode uint32, syserr syscall.Errno) {
	slog.Debug("Open() called", "path", node.fullPath)
	fh, fileMode, syserr := node.fuseDockerClient.open(ctx, node.fullPath, int(flags), fs.FileMode(mode))
	if syserr != 0 {
		slog.Error("remote error in open()", "path", node.fullPath, "errno", syserr)
	}
	mode = uint32(fileMode)
	return
}

func (node *Node) Readdir(ctx context.Context) (ds fusefs.DirStream, errno syscall.Errno) {
	slog.Debug("Readdir() called", "path", node.fullPath)
	ds, errno = node.fuseDockerClient.readDir(ctx, node.fullPath)
	if errno != 0 {
		slog.Error("remote error in readdir()", "path", node.fullPath, "errno", errno)
		return nil, errno
	}
	return
}

func (node *Node) Read(ctx context.Context, fh fusefs.FileHandle, dest []byte, off int64) (result fuse.ReadResult, syserr syscall.Errno) {
	slog.Debug("Read() called", "path", node.fullPath, "fh", fh, "off", off)

	data, syserr := node.fuseDockerClient.read(ctx, fh, off, len(dest))
	if syserr != 0 {
		slog.Error("remote error in read()", "fh", fh, "syserr", syserr)
		return
	}
	result = fuse.ReadResultData(data)
	return
}

func (node *Node) Readlink(ctx context.Context) (linkTarget []byte, errno syscall.Errno) {
	slog.Debug("Readlink() called", "path", node.fullPath)

	linkTarget, errno = node.fuseDockerClient.readlink(ctx, node.fullPath)
	if errno != 0 {
		slog.Error("remote error in readlink()", "path", node.fullPath, "errno", errno)
		return []byte{}, errno
	}
	return
}

func (node *Node) Rename(ctx context.Context, name string, newParent fusefs.InodeEmbedder, newName string, flags uint32) (errno syscall.Errno) {
	slog.Debug("Rename() called", "path", node.fullPath, "name", name, "newParent", newParent, "newName", newName, "flags", flags)

	fullPath := filepath.Clean(filepath.Join(node.fullPath, name))
	fullNewPath := filepath.Clean(filepath.Join(newParent.EmbeddedInode().Path(nil), newName))

	errno = node.fuseDockerClient.rename(ctx, fullPath, fullNewPath, flags)
	if errno != 0 {
		slog.Error("remote error in rename()", "path", node.fullPath, "errno", errno)
		return errno
	}
	return
}

func (node *Node) Rmdir(ctx context.Context, name string) (errno syscall.Errno) {
	slog.Debug("Rmdir() called", "path", node.fullPath, "name", name)

	fullPath := filepath.Clean(filepath.Join(node.fullPath, name))
	errno = node.fuseDockerClient.rmdir(ctx, fullPath)
	if errno != 0 {
		slog.Error("remote error in rmdir()", "path", node.fullPath, "errno", errno)
		return errno
	}
	return
}

func (node *Node) Setattr(ctx context.Context, f fusefs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) (errno syscall.Errno) {
	slog.Debug("Setattr() called", "path", node.fullPath, "in", *in)

	var fuseAttr statAttr
	errno = node.fuseDockerClient.setAttr(ctx, node.fullPath, in, &fuseAttr)
	if errno != 0 {
		slog.Error("remote error in setattr()", "path", node.fullPath, "errno", errno)
		return errno
	}
	out.Attr = fuseAttr.FuseAttr
	return
}

func (node *Node) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (newNode *fusefs.Inode, errno syscall.Errno) {
	slog.Debug("Symlink() called", "path", node.fullPath, "target", target, "name", name)

	newFullPath := filepath.Clean(filepath.Join(node.fullPath, name))
	errno = node.fuseDockerClient.symlink(ctx, target, newFullPath)
	if errno != 0 {
		slog.Error("remote error in symlink()", "path", node.fullPath, "errno", errno)
		return nil, errno
	}
	newNode = node.NewPersistentInode(ctx, NewNode(node.fuseDockerClient, newFullPath, node.fullPath), fusefs.StableAttr{Mode: fuse.S_IFLNK})
	return
}

func (node *Node) Unlink(ctx context.Context, name string) (errno syscall.Errno) {
	slog.Debug("Unlink() called", "path", node.fullPath, "name", name)

	fullPath := filepath.Clean(filepath.Join(node.fullPath, name))

	errno = node.fuseDockerClient.unlink(ctx, fullPath)
	if errno != 0 {
		slog.Error("remote error in unlink()", "path", node.fullPath, "errno", errno)
		return errno
	}
	return
}

func (node *Node) Write(ctx context.Context, fh fusefs.FileHandle, data []byte, off int64) (n uint32, syserr syscall.Errno) {
	slog.Debug("Write() called", "path", node.fullPath, "data", data, "off", off)

	ntmp, syserr := node.fuseDockerClient.write(ctx, fh, off, data)
	if syserr != 0 {
		slog.Error("remote error in write()", "path", node.fullPath, "syserr", syserr)
	}
	return uint32(ntmp), syserr
}
