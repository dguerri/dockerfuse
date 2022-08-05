package rpc_common

import (
	"io/fs"

	"github.com/hanwen/go-fuse/v2/fuse"
)

type DirEntry struct {
	Mode uint32
	Name string
	Ino  uint64
}

type ReadDirRequest struct {
	FullPath string
}
type ReadDirReply struct {
	DirEntries []DirEntry
}

type StatRequest struct {
	FullPath string
}
type StatReply struct {
	Mode       uint32
	Nlink      uint32
	Ino        uint64
	Uid        uint32
	Gid        uint32
	Atime      int64
	Mtime      int64
	Ctime      int64
	Size       int64
	Blocks     int64
	Blksize    int32
	LinkTarget string
}

type OpenRequest struct {
	FullPath string
	SAFlags  uint16 // System-agnostic flags
	Mode     fs.FileMode
}
type OpenReply struct {
	FD uintptr
	StatReply
}

type CloseRequest struct {
	FD uintptr
}
type CloseReply struct{}

type ReadRequest struct {
	FD     uintptr
	Offset int64
	Num    int
}
type ReadReply struct {
	Data []byte
}

type SeekRequest struct {
	FD     uintptr
	Offset int64
	Whence int
}
type SeekReply struct {
	Num int64
}

type WriteRequest struct {
	FD     uintptr
	Offset int64
	Data   []byte
}
type WriteReply struct {
	Num int
}

type UnlinkRequest struct {
	FullPath string
}
type UnlinkReply struct{}

type FsyncRequest struct {
	FD    uintptr
	Flags uint32
}
type FsyncReply struct{}

type MkdirRequest struct {
	FullPath string
	Mode     fs.FileMode
}
type MkdirReply StatReply

type RmdirRequest struct {
	FullPath string
}
type RmdirReply struct{}

type RenameRequest struct {
	FullPath    string
	FullNewPath string
	Flags       uint32
}
type RenameReply struct{}

type ReadlinkRequest struct {
	FullPath string
}
type ReadlinkReply struct {
	LinkTarget string
}

type LinkRequest struct {
	OldFullPath string
	NewFullPath string
}
type LinkReply struct{}

type SymlinkRequest struct {
	OldFullPath string
	NewFullPath string
}
type SymlinkReply struct{}

type SetAttrRequest struct {
	FullPath string
	AttrIn   fuse.SetAttrIn
}
type SetAttrReply StatReply
