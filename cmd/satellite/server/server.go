package server

import (
	"errors"
	"io/fs"
	"log"
	"os"
	"syscall"

	"dockerfuse/pkg/rpc_common"

	"github.com/hanwen/go-fuse/fuse"
	csys "github.com/lalkh/containerd/sys"
)

type DockerFuseFSOps struct {
	// Open file descriptors
	fds map[uintptr]file
}

func (d *DockerFuseFSOps) CloseAllFDs() {
	for k, fd := range d.fds {
		fd.Close()
		delete(d.fds, k)
	}
}

func NewDockerFuseFSOps() (fso *DockerFuseFSOps) {
	return &DockerFuseFSOps{
		fds: make(map[uintptr]file),
	}
}

func (fso *DockerFuseFSOps) Stat(request rpc_common.StatRequest, reply *rpc_common.StatReply) error {
	log.Printf("Stat called: %v", request)

	info, err := dfFS.Lstat(request.FullPath)
	if err != nil {
		return rpc_common.ErrnoToRPCErrorString(err)
	}
	sys := info.Sys().(*syscall.Stat_t)
	reply.Mode = uint32(sys.Mode)   // The int size of this is OS specific
	reply.Nlink = uint32(sys.Nlink) // 64bit on amd64, 32bit on arm64
	reply.Ino = sys.Ino
	reply.Uid = sys.Uid
	reply.Gid = sys.Gid
	reply.Atime = csys.StatAtime(sys).Sec // Workaround for os specific naming differences in Stat_t
	reply.Mtime = csys.StatMtime(sys).Sec
	reply.Ctime = csys.StatCtime(sys).Sec
	reply.Size = sys.Size
	reply.Blocks = sys.Blocks
	reply.Blksize = int32(sys.Blksize) // 64bit on amd64, 32bit on arm64
	reply.LinkTarget, err = dfFS.Readlink(request.FullPath)
	if err != nil {
		reply.LinkTarget = ""
	}
	return nil
}

func (fso *DockerFuseFSOps) ReadDir(request rpc_common.ReadDirRequest, reply *rpc_common.ReadDirReply) error {
	log.Printf("ReadDir called: %v", request)

	files, err := dfFS.ReadDir(request.FullPath)
	if err != nil {
		return rpc_common.ErrnoToRPCErrorString(err)
	}

	reply.DirEntries = make([]rpc_common.DirEntry, 0, len(files))
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue // File has been removed since directory read, skip it
			} else {
				log.Printf("Unexpected file.Info() error: %v", err)
				return rpc_common.ErrnoToRPCErrorString(syscall.EIO)
			}
		}
		sys := *(info.Sys().(*syscall.Stat_t))
		entry := rpc_common.DirEntry{
			Ino:  sys.Ino,
			Name: file.Name(),
			Mode: uint32(sys.Mode), // The int size of this is OS specific
		}
		reply.DirEntries = append(reply.DirEntries, entry)
	}
	return nil
}

func (fso *DockerFuseFSOps) Open(request rpc_common.OpenRequest, reply *rpc_common.OpenReply) error {
	log.Printf("Open called: %v", request)

	fd, err := dfFS.OpenFile(request.FullPath, rpc_common.SAFlagsToSystem(request.SAFlags), request.Mode)
	if err != nil {
		return rpc_common.ErrnoToRPCErrorString(err)
	}

	uintptr_fd := fd.Fd()
	if fd, ok := fso.fds[uintptr_fd]; ok {
		fd.Close() // Make sure we don't leak stale FDs
	}
	fso.fds[uintptr_fd] = fd

	info, err := fd.Stat()
	if err != nil {
		return rpc_common.ErrnoToRPCErrorString(err)
	}

	sys := info.Sys().(*syscall.Stat_t)
	reply.Mode = uint32(sys.Mode)   // The int size of this is OS specific
	reply.Nlink = uint32(sys.Nlink) // 64bit on amd64, 32bit on arm64
	reply.Ino = sys.Ino
	reply.Uid = sys.Uid
	reply.Gid = sys.Gid
	reply.Atime = csys.StatAtime(sys).Sec // Workaround for os specific naming differences in Stat_t
	reply.Mtime = csys.StatMtime(sys).Sec
	reply.Ctime = csys.StatCtime(sys).Sec
	reply.Size = sys.Size
	reply.Blocks = sys.Blocks
	reply.Blksize = int32(sys.Blksize) // 64bit on amd64, 32bit on arm64
	reply.LinkTarget, err = dfFS.Readlink(request.FullPath)
	if err != nil {
		reply.LinkTarget = ""
	}
	reply.FD = uintptr_fd
	return nil
}

func (fso *DockerFuseFSOps) Close(request rpc_common.CloseRequest, reply *rpc_common.CloseReply) error {
	log.Printf("Close called: %v", request)

	fd, ok := fso.fds[request.FD]
	if !ok {
		return rpc_common.ErrnoToRPCErrorString(syscall.EINVAL)
	}
	defer delete(fso.fds, request.FD)
	err := fd.Close()
	if err != nil {
		return rpc_common.ErrnoToRPCErrorString(err)
	}

	return nil
}

func (fso *DockerFuseFSOps) Read(request rpc_common.ReadRequest, reply *rpc_common.ReadReply) error {
	log.Printf("Read called: %v", request)

	file, ok := fso.fds[request.FD]
	if !ok {
		return rpc_common.ErrnoToRPCErrorString(syscall.EINVAL)
	}

	data := make([]byte, request.Num)
	n, err := file.ReadAt(data, request.Offset)
	if err != nil {
		return rpc_common.ErrnoToRPCErrorString(err)
	}

	reply.Data = data[:n]
	return nil
}

func (fso *DockerFuseFSOps) Seek(request rpc_common.SeekRequest, reply *rpc_common.SeekReply) error {
	log.Printf("Seek called: %v", request)

	file, ok := fso.fds[request.FD]
	if !ok {
		return rpc_common.ErrnoToRPCErrorString(syscall.EINVAL)
	}

	n, err := file.Seek(request.Offset, request.Whence)
	if err != nil {
		return rpc_common.ErrnoToRPCErrorString(err)
	}

	reply.Num = n
	return nil
}

func (fso *DockerFuseFSOps) Write(request rpc_common.WriteRequest, reply *rpc_common.WriteReply) error {
	log.Printf("Write called: %v", request)

	file, ok := fso.fds[request.FD]
	if !ok {
		return rpc_common.ErrnoToRPCErrorString(syscall.EINVAL)
	}

	var err error
	var n int
	if request.Offset > 0 {
		n, err = file.WriteAt(request.Data, request.Offset)

	} else {
		n, err = file.Write(request.Data)

	}
	if err != nil {
		return rpc_common.ErrnoToRPCErrorString(err)
	}

	reply.Num = n
	return nil
}

func (fso *DockerFuseFSOps) Unlink(request rpc_common.UnlinkRequest, reply *rpc_common.UnlinkReply) error {
	log.Printf("Unlink called: %v", request)

	err := os.Remove(request.FullPath)
	if err != nil {
		return rpc_common.ErrnoToRPCErrorString(err)
	}
	return nil
}

func (fso *DockerFuseFSOps) Fsync(request rpc_common.FsyncRequest, reply *rpc_common.FsyncReply) error {
	log.Printf("Fsync called: %v", request)

	file, ok := fso.fds[request.FD]
	if !ok {
		return rpc_common.ErrnoToRPCErrorString(syscall.EINVAL)
	}

	err := file.Sync()
	if err != nil {
		return rpc_common.ErrnoToRPCErrorString(err)
	}
	return nil
}

func (fso *DockerFuseFSOps) Mkdir(request rpc_common.MkdirRequest, reply *rpc_common.MkdirReply) error {
	log.Printf("Mkdir called: %v", request)

	err := os.Mkdir(request.FullPath, os.FileMode(request.Mode))
	if err != nil {
		return rpc_common.ErrnoToRPCErrorString(err)
	}

	err = fso.Stat(rpc_common.StatRequest{FullPath: request.FullPath}, (*rpc_common.StatReply)(reply))
	if err != nil {
		return err
	}
	return nil
}

func (fso *DockerFuseFSOps) Rmdir(request rpc_common.RmdirRequest, reply *rpc_common.RmdirReply) error {
	log.Printf("Rmdir called: %v", request)

	err := os.Remove(request.FullPath)
	if err != nil {
		return rpc_common.ErrnoToRPCErrorString(err)
	}
	return nil
}

const (
	RENAME_NOREPLACE = 1
	RENAME_EXCHANGE  = 2
	RENAME_WHITEOUT  = 4
)

func (fso *DockerFuseFSOps) Rename(request rpc_common.RenameRequest, reply *rpc_common.RenameReply) error {
	log.Printf("Rename called: %v", request)

	err := os.Rename(request.FullPath, request.FullNewPath)
	if err != nil {
		return rpc_common.ErrnoToRPCErrorString(err)
	}

	return nil
}

func (fso *DockerFuseFSOps) Readlink(request rpc_common.ReadlinkRequest, reply *rpc_common.ReadlinkReply) error {
	log.Printf("Readlink called: %v", request)

	linkTarget, err := os.Readlink(request.FullPath)
	if err != nil {
		return rpc_common.ErrnoToRPCErrorString(err)
	}
	reply.LinkTarget = linkTarget
	return nil
}

func (fso *DockerFuseFSOps) Link(request rpc_common.LinkRequest, reply *rpc_common.LinkReply) error {
	log.Printf("Link called: %v", request)

	err := os.Link(request.OldFullPath, request.NewFullPath)
	if err != nil {
		return rpc_common.ErrnoToRPCErrorString(err)
	}

	return nil
}

func (fso *DockerFuseFSOps) Symlink(request rpc_common.SymlinkRequest, reply *rpc_common.SymlinkReply) error {
	log.Printf("Symlink called: %v", request)

	err := os.Symlink(request.OldFullPath, request.NewFullPath)
	if err != nil {
		return rpc_common.ErrnoToRPCErrorString(err)
	}

	return nil
}

func (fso *DockerFuseFSOps) SetAttr(request rpc_common.SetAttrRequest, reply *rpc_common.SetAttrReply) (err error) {
	log.Printf("SetAttr called: %v", request)

	// Set Mode
	if m, ok := request.AttrIn.GetMode(); ok {
		if err := syscall.Chmod(request.FullPath, m); err != nil {
			return rpc_common.ErrnoToRPCErrorString(err)
		}
	}

	// Set Owner/Group
	uid, uok := request.AttrIn.GetUID()
	gid, gok := request.AttrIn.GetGID()
	if uok || gok {
		suid := -1
		sgid := -1
		if uok {
			suid = int(uid)
		}
		if gok {
			sgid = int(gid)
		}
		if err := syscall.Chown(request.FullPath, suid, sgid); err != nil {
			return rpc_common.ErrnoToRPCErrorString(err)
		}
	}

	// Set A/M-Time
	mtime, mok := request.AttrIn.GetMTime()
	atime, aok := request.AttrIn.GetATime()
	if mok || aok {
		ap := &atime
		mp := &mtime
		if !aok {
			ap = nil
		}
		if !mok {
			mp = nil
		}
		var ts [2]syscall.Timespec
		ts[0] = fuse.UtimeToTimespec(ap)
		ts[1] = fuse.UtimeToTimespec(mp)

		if err := syscall.UtimesNano(request.FullPath, ts[:]); err != nil {
			return rpc_common.ErrnoToRPCErrorString(err)
		}
	}

	// Set size
	if sz, ok := request.AttrIn.GetSize(); ok {
		if err := syscall.Truncate(request.FullPath, int64(sz)); err != nil {
			return rpc_common.ErrnoToRPCErrorString(err)
		}
	}

	err = fso.Stat(rpc_common.StatRequest{FullPath: request.FullPath}, (*rpc_common.StatReply)(reply))
	if err != nil {
		return err
	}
	return nil
}
