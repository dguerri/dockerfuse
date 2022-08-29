package rpccommon

import (
	"os"
	"time"
)

// DirEntry holds essential information about an entry in a directory
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
	FD       uintptr
	UseFD    bool
}
type StatReply struct {
	Mode       uint32
	Nlink      uint32
	Ino        uint64
	UID        uint32
	GID        uint32
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
	Mode     os.FileMode
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
	Mode     os.FileMode
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

const ( // bitmap for SetAttrRequest.ValidAttrs
	SATTR_ATIME = (1 << 0)
	SATTR_GID   = (1 << 1)
	SATTR_MODE  = (1 << 2)
	SATTR_MTIME = (1 << 3)
	SATTR_SIZE  = (1 << 4)
	SATTR_UID   = (1 << 5)
)
const UTIME_OMIT = ((1 << 30) - 2)

type SetAttrRequest struct {
	FullPath   string
	ValidAttrs uint32 // See SATTR_* bitmap
	ATime      time.Time
	MTime      time.Time
	UID        uint32
	GID        uint32
	Mode       uint32
	Size       uint64
}
type SetAttrReply StatReply

func (r *SetAttrRequest) GetMode() (m uint32, ok bool) {
	if r.ValidAttrs&SATTR_MODE == SATTR_MODE {
		return r.Mode, true
	}
	return 0, false
}
func (r *SetAttrRequest) GetUID() (u uint32, ok bool) {
	if r.ValidAttrs&SATTR_UID == SATTR_UID {
		return r.UID, true
	}
	return 0, false
}
func (r *SetAttrRequest) GetGID() (g uint32, ok bool) {
	if r.ValidAttrs&SATTR_GID == SATTR_GID {
		return r.GID, true
	}
	return 0, false
}
func (r *SetAttrRequest) GetATime() (m time.Time, ok bool) {
	if r.ValidAttrs&SATTR_ATIME == SATTR_ATIME {
		return r.ATime, true
	}
	return time.Time{}, false
}
func (r *SetAttrRequest) GetMTime() (m time.Time, ok bool) {
	if r.ValidAttrs&SATTR_MTIME == SATTR_MTIME {
		return r.MTime, true
	}
	return time.Time{}, false
}
func (r *SetAttrRequest) GetSize() (m uint64, ok bool) {
	if r.ValidAttrs&SATTR_SIZE == SATTR_SIZE {
		return r.Size, true
	}
	return 0, false
}

func (r *SetAttrRequest) SetMode(m uint32)     { r.Mode = m; r.ValidAttrs |= SATTR_MODE }
func (r *SetAttrRequest) SetUID(u uint32)      { r.UID = u; r.ValidAttrs |= SATTR_UID }
func (r *SetAttrRequest) SetGID(g uint32)      { r.GID = g; r.ValidAttrs |= SATTR_GID }
func (r *SetAttrRequest) SetATime(a time.Time) { r.ATime = a; r.ValidAttrs |= SATTR_ATIME }
func (r *SetAttrRequest) SetMTime(m time.Time) { r.MTime = m; r.ValidAttrs |= SATTR_MTIME }
func (r *SetAttrRequest) SetSize(s uint64)     { r.Size = s; r.ValidAttrs |= SATTR_SIZE }
