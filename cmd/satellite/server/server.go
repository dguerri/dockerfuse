package server

import (
	"errors"
	"io/fs"
	"log"
	"os"
	"syscall"

	"github.com/dguerri/dockerfuse/pkg/rpccommon"
	csys "github.com/lalkh/containerd/sys"
)

// DockerFuseFSOps is used to interact with the filesystem
type DockerFuseFSOps struct {
	// Open file descriptors
	fds map[uintptr]file
}

// NewDockerFuseFSOps returns a new DockerFuseFSOps
func NewDockerFuseFSOps() (fso *DockerFuseFSOps) {
	return &DockerFuseFSOps{
		fds: make(map[uintptr]file),
	}
}

// CloseAllFDs closes all file descriptors currently opened by the server.
func (fso *DockerFuseFSOps) CloseAllFDs() {
	for k, fd := range fso.fds {
		fd.Close()
		delete(fso.fds, k)
	}
}

// Stat returns file information about the requested path.
func (fso *DockerFuseFSOps) Stat(request rpccommon.StatRequest, reply *rpccommon.StatReply) error {
	log.Printf("Stat called: %v", request)

	var info fs.FileInfo
	var err error

	info, err = dfFS.Lstat(request.FullPath)
	if err != nil {
		return rpccommon.ErrnoToRPCErrorString(err)
	}

	sys := info.Sys().(*syscall.Stat_t)
	reply.Mode = uint32(sys.Mode)   // The int size of this is OS specific
	reply.Nlink = uint32(sys.Nlink) // 64bit on amd64, 32bit on arm64
	reply.Ino = sys.Ino
	reply.UID = sys.Uid
	reply.GID = sys.Gid
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

// ReadDir lists the contents of a directory.
func (fso *DockerFuseFSOps) ReadDir(request rpccommon.ReadDirRequest, reply *rpccommon.ReadDirReply) error {
	log.Printf("ReadDir called: %v", request)

	files, err := dfFS.ReadDir(request.FullPath)
	if err != nil {
		return rpccommon.ErrnoToRPCErrorString(err)
	}

	reply.DirEntries = make([]rpccommon.DirEntry, 0, len(files))
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue // File has been removed since directory read, skip it
			} else {
				log.Printf("Unexpected file.Info() error: %v", err)
				return rpccommon.ErrnoToRPCErrorString(syscall.EIO)
			}
		}
		sys := *(info.Sys().(*syscall.Stat_t))
		entry := rpccommon.DirEntry{
			Ino:  sys.Ino,
			Name: file.Name(),
			Mode: uint32(sys.Mode), // The int size of this is OS specific
		}
		reply.DirEntries = append(reply.DirEntries, entry)
	}
	return nil
}

// Open opens the requested file and stores the resulting descriptor.
func (fso *DockerFuseFSOps) Open(request rpccommon.OpenRequest, reply *rpccommon.OpenReply) error {
	log.Printf("Open called: %v", request)

	fd, err := dfFS.OpenFile(request.FullPath, rpccommon.SAFlagsToSystem(request.SAFlags), request.Mode)
	if err != nil {
		return rpccommon.ErrnoToRPCErrorString(err)
	}

	uintptrFD := fd.Fd()
	if fd, ok := fso.fds[uintptrFD]; ok {
		fd.Close() // Make sure we don't leak stale FDs
	}
	fso.fds[uintptrFD] = fd

	info, err := fd.Stat()
	if err != nil {
		return rpccommon.ErrnoToRPCErrorString(err)
	}

	sys := info.Sys().(*syscall.Stat_t)
	reply.Mode = uint32(sys.Mode)   // The int size of this is OS specific
	reply.Nlink = uint32(sys.Nlink) // 64bit on amd64, 32bit on arm64
	reply.Ino = sys.Ino
	reply.UID = sys.Uid
	reply.GID = sys.Gid
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
	reply.FD = uintptrFD
	return nil
}

// Close closes an open file descriptor previously returned by Open.
func (fso *DockerFuseFSOps) Close(request rpccommon.CloseRequest, reply *rpccommon.CloseReply) error {
	log.Printf("Close called: %v", request)

	fd, ok := fso.fds[request.FD]
	if !ok {
		return rpccommon.ErrnoToRPCErrorString(syscall.EINVAL)
	}
	defer delete(fso.fds, request.FD)
	err := fd.Close()
	if err != nil {
		return rpccommon.ErrnoToRPCErrorString(err)
	}

	return nil
}

// Read reads data from an open file descriptor.
func (fso *DockerFuseFSOps) Read(request rpccommon.ReadRequest, reply *rpccommon.ReadReply) error {
	log.Printf("Read called: %v", request)

	file, ok := fso.fds[request.FD]
	if !ok {
		return rpccommon.ErrnoToRPCErrorString(syscall.EINVAL)
	}

	data := make([]byte, request.Num)
	n, err := file.ReadAt(data, request.Offset)
	if err != nil && (err.Error() != "EOF" || n <= 0) {
		return rpccommon.ErrnoToRPCErrorString(err)
	}

	reply.Data = data[:n]
	return nil
}

// Seek moves the file offset associated with a descriptor.
func (fso *DockerFuseFSOps) Seek(request rpccommon.SeekRequest, reply *rpccommon.SeekReply) error {
	log.Printf("Seek called: %v", request)

	file, ok := fso.fds[request.FD]
	if !ok {
		return rpccommon.ErrnoToRPCErrorString(syscall.EINVAL)
	}

	n, err := file.Seek(request.Offset, request.Whence)
	if err != nil {
		return rpccommon.ErrnoToRPCErrorString(err)
	}

	reply.Num = n
	return nil
}

// Write writes data to an open file descriptor.
func (fso *DockerFuseFSOps) Write(request rpccommon.WriteRequest, reply *rpccommon.WriteReply) error {
	log.Printf("Write called: %v", request)

	file, ok := fso.fds[request.FD]
	if !ok {
		return rpccommon.ErrnoToRPCErrorString(syscall.EINVAL)
	}

	n, err := file.WriteAt(request.Data, request.Offset)
	if err != nil {
		return rpccommon.ErrnoToRPCErrorString(err)
	}

	reply.Num = n
	return nil
}

// Unlink removes a file from the filesystem.
func (fso *DockerFuseFSOps) Unlink(request rpccommon.UnlinkRequest, reply *rpccommon.UnlinkReply) error {
	log.Printf("Unlink called: %v", request)

	err := dfFS.Remove(request.FullPath)
	if err != nil {
		return rpccommon.ErrnoToRPCErrorString(err)
	}
	return nil
}

// Fsync flushes pending file changes to disk.
func (fso *DockerFuseFSOps) Fsync(request rpccommon.FsyncRequest, reply *rpccommon.FsyncReply) error {
	log.Printf("Fsync called: %v", request)

	file, ok := fso.fds[request.FD]
	if !ok {
		return rpccommon.ErrnoToRPCErrorString(syscall.EINVAL)
	}

	err := file.Sync()
	if err != nil {
		return rpccommon.ErrnoToRPCErrorString(err)
	}
	return nil
}

// Mkdir creates a new directory.
func (fso *DockerFuseFSOps) Mkdir(request rpccommon.MkdirRequest, reply *rpccommon.MkdirReply) error {
	log.Printf("Mkdir called: %v", request)

	err := dfFS.Mkdir(request.FullPath, os.FileMode(request.Mode))
	if err != nil {
		return rpccommon.ErrnoToRPCErrorString(err)
	}

	err = fso.Stat(rpccommon.StatRequest{FullPath: request.FullPath}, (*rpccommon.StatReply)(reply))
	if err != nil {
		return err
	}
	return nil
}

// Rmdir removes a directory from the filesystem.
func (fso *DockerFuseFSOps) Rmdir(request rpccommon.RmdirRequest, reply *rpccommon.RmdirReply) error {
	log.Printf("Rmdir called: %v", request)

	err := dfFS.Remove(request.FullPath)
	if err != nil {
		return rpccommon.ErrnoToRPCErrorString(err)
	}
	return nil
}

// Rename renames a file or directory.
func (fso *DockerFuseFSOps) Rename(request rpccommon.RenameRequest, reply *rpccommon.RenameReply) error {
	log.Printf("Rename called: %v", request)

	err := dfFS.Rename(request.FullPath, request.FullNewPath)
	if err != nil {
		return rpccommon.ErrnoToRPCErrorString(err)
	}

	return nil
}

// Readlink returns the target of a symbolic link.
func (fso *DockerFuseFSOps) Readlink(request rpccommon.ReadlinkRequest, reply *rpccommon.ReadlinkReply) error {
	log.Printf("Readlink called: %v", request)

	linkTarget, err := dfFS.Readlink(request.FullPath)
	if err != nil {
		return rpccommon.ErrnoToRPCErrorString(err)
	}
	reply.LinkTarget = linkTarget
	return nil
}

// Link creates a new hard link.
func (fso *DockerFuseFSOps) Link(request rpccommon.LinkRequest, reply *rpccommon.LinkReply) error {
	log.Printf("Link called: %v", request)

	err := dfFS.Link(request.OldFullPath, request.NewFullPath)
	if err != nil {
		return rpccommon.ErrnoToRPCErrorString(err)
	}

	return nil
}

// Symlink creates a new symbolic link.
func (fso *DockerFuseFSOps) Symlink(request rpccommon.SymlinkRequest, reply *rpccommon.SymlinkReply) error {
	log.Printf("Symlink called: %v", request)

	err := dfFS.Symlink(request.OldFullPath, request.NewFullPath)
	if err != nil {
		return rpccommon.ErrnoToRPCErrorString(err)
	}

	return nil
}

// SetAttr changes file attributes like mode, owner or timestamps.
func (fso *DockerFuseFSOps) SetAttr(request rpccommon.SetAttrRequest, reply *rpccommon.SetAttrReply) (err error) {
	log.Printf("SetAttr called: %v", request)

	// Set Mode
	if m, ok := request.GetMode(); ok {
		if err := dfFS.Chmod(request.FullPath, os.FileMode(m)); err != nil {
			return rpccommon.ErrnoToRPCErrorString(err)
		}
	}

	// Set Owner/Group
	uid, uok := request.GetUID()
	gid, gok := request.GetGID()
	if uok || gok {
		suid := -1
		sgid := -1
		if uok {
			suid = int(uid)
		}
		if gok {
			sgid = int(gid)
		}
		if err := dfFS.Chown(request.FullPath, suid, sgid); err != nil {
			return rpccommon.ErrnoToRPCErrorString(err)
		}
	}

	// Set A/M-Time
	atime, aok := request.GetATime()
	mtime, mok := request.GetMTime()
	if mok || aok {
		var ts [2]syscall.Timespec
		if aok {
			ts[0] = syscall.NsecToTimespec(atime.UnixNano())
		} else {
			ts[0].Nsec = rpccommon.UTIME_OMIT
		}
		if mok {
			ts[1] = syscall.NsecToTimespec(mtime.UnixNano())
		} else {
			ts[1].Nsec = rpccommon.UTIME_OMIT
		}

		if err := dfFS.UtimesNano(request.FullPath, ts[:]); err != nil {
			return rpccommon.ErrnoToRPCErrorString(err)
		}
	}

	// Set size
	if sz, ok := request.GetSize(); ok {
		if err := dfFS.Truncate(request.FullPath, int64(sz)); err != nil {
			return rpccommon.ErrnoToRPCErrorString(err)
		}
	}

	err = fso.Stat(rpccommon.StatRequest{FullPath: request.FullPath}, (*rpccommon.StatReply)(reply))
	if err != nil {
		return err
	}
	return nil
}
