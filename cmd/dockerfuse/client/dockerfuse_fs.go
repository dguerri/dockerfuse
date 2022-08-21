package client

import (
	"context"
	"io/fs"
	"log"
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

type Node struct {
	fusefs.Inode

	Data             []byte
	fullPath         string
	fuseDockerClient FuseDockerClientInterface
}

func NewNode(fuseDockerClient FuseDockerClientInterface, fullPath string, linkTarget string) *Node {
	return &Node{
		Data:             []byte(linkTarget),
		fullPath:         fullPath,
		fuseDockerClient: fuseDockerClient,
	}
}

func (node *Node) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (newNode *fusefs.Inode, fh fusefs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	log.Printf("Create() called on '%s' name: '%s', flags: %d, mode: %d", node.fullPath, name, flags, mode)

	fullPath := filepath.Clean(filepath.Join(node.fullPath, name))
	var fuseAttr StatAttr
	fh, errno = node.fuseDockerClient.create(ctx, fullPath, int(flags), fs.FileMode(mode), &fuseAttr)
	if errno != 0 {
		log.Printf("error in open for '%s': %d", fullPath, errno)
		return nil, nil, 0, errno
	}
	fuseFlags = flags
	out.Attr = fuseAttr.FuseAttr
	newNode = node.NewPersistentInode(ctx, NewNode(node.fuseDockerClient, fullPath, ""), fusefs.StableAttr{Ino: out.Ino})
	return
}

func (node *Node) Flush(ctx context.Context, fh fusefs.FileHandle) (syserr syscall.Errno) {
	log.Printf("Flush on file %s, fh: '%v'", node.fullPath, fh)

	return node.fuseDockerClient.close(ctx, fh)
}

func (node *Node) Release(ctx context.Context, fh fusefs.FileHandle) (syserr syscall.Errno) {
	log.Printf("Release on file %s, fh: '%v'", node.fullPath, fh)

	return node.fuseDockerClient.close(ctx, fh)
}

func (node *Node) Fsync(ctx context.Context, fh fusefs.FileHandle, flags uint32) (syserr syscall.Errno) {
	log.Printf("Fsync on file %s, fh: '%v'", node.fullPath, fh)

	return node.fuseDockerClient.fsync(ctx, fh, flags)
}

func (node *Node) Getattr(ctx context.Context, fh fusefs.FileHandle, out *fuse.AttrOut) (errno syscall.Errno) {
	log.Printf("Getattr() called on '%s' (fh: %v)", node.fullPath, fh)

	var fuseAttr StatAttr
	errno = node.fuseDockerClient.stat(ctx, node.fullPath, fh, &fuseAttr)
	if errno != 0 {
		log.Printf("error in stat for '%s': %d", node.fullPath, errno)
		return
	}

	out.Attr = fuseAttr.FuseAttr
	return
}

func (node *Node) Link(ctx context.Context, target fusefs.InodeEmbedder, name string, out *fuse.EntryOut) (newNode *fusefs.Inode, errno syscall.Errno) {
	log.Printf("Link() called on '%s' target: '%v' name: '%s'", node.fullPath, target, name)

	newFullPath := filepath.Clean(filepath.Join(node.fullPath, name))
	errno = node.fuseDockerClient.link(ctx, target.(*Node).fullPath, newFullPath)
	if errno != 0 {
		log.Printf("error in link for '%s': %d", node.fullPath, errno)
		return nil, errno
	}

	newNode = node.NewPersistentInode(ctx, NewNode(node.fuseDockerClient, newFullPath, ""), fusefs.StableAttr{Mode: target.EmbeddedInode().Mode()})
	return
}

func (node *Node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (n *fusefs.Inode, syserr syscall.Errno) {
	log.Printf("Lookup() called on '%s' with name '%s'", node.fullPath, name)
	fullPath := filepath.Clean(filepath.Join(node.fullPath, name))

	var fuseAttr StatAttr
	syserr = node.fuseDockerClient.stat(ctx, fullPath, nil, &fuseAttr)
	if syserr != 0 {
		log.Printf("error in stat for '%s': %d", fullPath, syserr)
		return nil, syserr
	}

	out.Attr = fuseAttr.FuseAttr

	stableAttr := fusefs.StableAttr{}
	switch {
	case out.Attr.Mode&fuse.S_IFDIR == fuse.S_IFDIR:
		log.Printf("adding dir '%s'", fullPath)
		stableAttr.Mode = fuse.S_IFDIR
	case out.Attr.Mode&fuse.S_IFLNK == fuse.S_IFLNK:
		log.Printf("adding symlink '%s'", fullPath)
		stableAttr.Mode = fuse.S_IFLNK
	case out.Attr.Mode&fuse.S_IFIFO == fuse.S_IFIFO:
		log.Printf("adding FIFO '%s'", fullPath)
		stableAttr.Mode = fuse.S_IFIFO
	default:
		log.Printf("adding reg '%s'", fullPath)
		stableAttr.Mode = fuse.S_IFREG
	}

	return node.NewPersistentInode(ctx, NewNode(node.fuseDockerClient, fullPath, fuseAttr.LinkTarget), stableAttr), 0
}

func (node *Node) Lseek(ctx context.Context, fh fusefs.FileHandle, off uint64, whence uint32) (n uint64, syserr syscall.Errno) {
	log.Printf("Lseek() (Node) called on '%s'", node.fullPath)

	ntmp, syserr := node.fuseDockerClient.seek(ctx, fh, int64(off), int(whence))
	return uint64(ntmp), syserr
}

func (node *Node) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (newNode *fusefs.Inode, errno syscall.Errno) {
	log.Printf("Mkdir() called on '%s' name: '%s', mode: %d", node.fullPath, name, mode)

	fullPath := filepath.Clean(filepath.Join(node.fullPath, name))
	var fuseAttr StatAttr
	errno = node.fuseDockerClient.mkdir(ctx, fullPath, fs.FileMode(mode), &fuseAttr)
	if errno != 0 {
		log.Printf("error in mkdir for '%s': %d", fullPath, errno)
		return nil, errno
	}
	out.Attr = fuseAttr.FuseAttr
	newNode = node.NewPersistentInode(ctx, NewNode(node.fuseDockerClient, fullPath, ""), fusefs.StableAttr{Mode: fuse.S_IFDIR, Ino: out.Ino})
	return
}

func (node *Node) Open(ctx context.Context, flags uint32) (fh fusefs.FileHandle, mode uint32, syserr syscall.Errno) {
	log.Printf("Open on file %s", node.fullPath)
	fh, fileMode, syserr := node.fuseDockerClient.open(ctx, node.fullPath, int(flags), fs.FileMode(mode))
	mode = uint32(fileMode)
	return
}

func (node *Node) Readdir(ctx context.Context) (ds fusefs.DirStream, errno syscall.Errno) {
	log.Printf("Readdir() called on '%s'", node.fullPath)
	ds, errno = node.fuseDockerClient.readDir(ctx, node.fullPath)
	if errno != 0 {
		log.Printf("error in readdir for '%s': %d", node.fullPath, errno)
		return nil, errno
	}
	return
}

func (node *Node) Read(ctx context.Context, fh fusefs.FileHandle, dest []byte, off int64) (result fuse.ReadResult, syserr syscall.Errno) {
	log.Printf("Read on file %s, fh: '%v', off: %d", node.fullPath, fh, off)

	data, syserr := node.fuseDockerClient.read(ctx, fh, off, len(dest))
	if syserr != 0 {
		return
	}

	result = fuse.ReadResultData(data)
	return
}

func (node *Node) Readlink(ctx context.Context) (linkTarget []byte, errno syscall.Errno) {
	log.Printf("Readlink() called on '%s'", node.fullPath)

	linkTarget, errno = node.fuseDockerClient.readlink(ctx, node.fullPath)
	if errno != 0 {
		log.Printf("error in readlink for '%s': %d", node.fullPath, errno)
		return []byte{}, errno
	}

	return
}

func (node *Node) Rename(ctx context.Context, name string, newParent fusefs.InodeEmbedder, newName string, flags uint32) (errno syscall.Errno) {
	log.Printf("Rename() called on '%s' name: '%s' newparent: '%v', newname: '%s' flags: %d", node.fullPath, name, newParent, newName, flags)

	fullPath := filepath.Clean(filepath.Join(node.fullPath, name))
	fullNewPath := filepath.Clean(filepath.Join(newParent.EmbeddedInode().Path(nil), newName))

	errno = node.fuseDockerClient.rename(ctx, fullPath, fullNewPath, flags)
	if errno != 0 {
		log.Printf("error in rename for '%s': %d", fullPath, errno)
		return errno
	}

	return
}

func (node *Node) Rmdir(ctx context.Context, name string) (errno syscall.Errno) {
	log.Printf("Rmdir() called on '%s' name: '%s'", node.fullPath, name)

	fullPath := filepath.Clean(filepath.Join(node.fullPath, name))
	errno = node.fuseDockerClient.rmdir(ctx, fullPath)
	if errno != 0 {
		log.Printf("error in rmdir for '%s': %d", fullPath, errno)
		return errno
	}

	return
}

func (node *Node) Setattr(ctx context.Context, f fusefs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) (errno syscall.Errno) {
	log.Printf("Setattr() called on '%s' with %v", node.fullPath, *in)

	var fuseAttr StatAttr
	errno = node.fuseDockerClient.setAttr(ctx, node.fullPath, in, &fuseAttr)
	if errno != 0 {
		log.Printf("error in setattr for '%s': %d", node.fullPath, errno)
		return errno
	}

	out.Attr = fuseAttr.FuseAttr
	return
}

func (node *Node) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (newNode *fusefs.Inode, errno syscall.Errno) {
	log.Printf("Symlink() called on '%s' target: '%s' name: '%s'", node.fullPath, target, name)

	newFullPath := filepath.Clean(filepath.Join(node.fullPath, name))
	errno = node.fuseDockerClient.symlink(ctx, target, newFullPath)
	if errno != 0 {
		log.Printf("error in symlink for '%s': %d", node.fullPath, errno)
		return nil, errno
	}

	newNode = node.NewPersistentInode(ctx, NewNode(node.fuseDockerClient, newFullPath, node.fullPath), fusefs.StableAttr{Mode: fuse.S_IFLNK})
	return
}

func (node *Node) Unlink(ctx context.Context, name string) (errno syscall.Errno) {
	log.Printf("Unlink() called on '%s' name: '%s'", node.fullPath, name)

	fullPath := filepath.Clean(filepath.Join(node.fullPath, name))

	errno = node.fuseDockerClient.unlink(ctx, fullPath)
	if errno != 0 {
		log.Printf("error in unlink for '%s': %d", fullPath, errno)
		return errno
	}
	return
}

func (node *Node) Write(ctx context.Context, fh fusefs.FileHandle, data []byte, off int64) (n uint32, syserr syscall.Errno) {
	log.Printf("Write() called on '%v', data: %v, off: %d", node, data, off)

	ntmp, syserr := node.fuseDockerClient.write(ctx, fh, off, data)
	return uint32(ntmp), syserr

}
