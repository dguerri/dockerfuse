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

// ReadDirRequest describes a request to read a directory.
type ReadDirRequest struct {
	FullPath string
}

// ReadDirReply is the reply to a ReadDirRequest.
type ReadDirReply struct {
	DirEntries []DirEntry
}

// StatRequest contains the path information for a stat call.
type StatRequest struct {
	FullPath string
}

// StatReply mirrors os.Stat results returned via RPC.
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

// OpenRequest represents an open file request.
type OpenRequest struct {
	FullPath string
	SAFlags  uint16 // System-agnostic flags
	Mode     os.FileMode
}

// OpenReply contains information about the opened file.
type OpenReply struct {
	FD uintptr
	StatReply
}

// CloseRequest identifies a file descriptor to close.
type CloseRequest struct {
	FD uintptr
}

// CloseReply is returned on a successful close.
type CloseReply struct{}

// ReadRequest represents a remote read request.
type ReadRequest struct {
	FD     uintptr
	Offset int64
	Num    int
}

// ReadReply contains the bytes read from a file.
type ReadReply struct {
	Data []byte
}

// SeekRequest represents a seek operation on a file.
type SeekRequest struct {
	FD     uintptr
	Offset int64
	Whence int
}

// SeekReply carries the resulting offset after a seek.
type SeekReply struct {
	Num int64
}

// WriteRequest represents a write operation.
type WriteRequest struct {
	FD     uintptr
	Offset int64
	Data   []byte
}

// WriteReply contains the number of bytes written.
type WriteReply struct {
	Num int
}

// UnlinkRequest specifies the path of the file to remove.
type UnlinkRequest struct {
	FullPath string
}

// UnlinkReply is returned on a successful unlink.
type UnlinkReply struct{}

// FsyncRequest represents a fsync call.
type FsyncRequest struct {
	FD    uintptr
	Flags uint32
}

// FsyncReply is returned on a successful fsync.
type FsyncReply struct{}

// MkdirRequest represents a directory creation request.
type MkdirRequest struct {
	FullPath string
	Mode     os.FileMode
}

// MkdirReply contains attributes of the newly created directory.
type MkdirReply StatReply

// RmdirRequest identifies the directory to remove.
type RmdirRequest struct {
	FullPath string
}

// RmdirReply is returned on a successful rmdir.
type RmdirReply struct{}

// RenameRequest contains old and new names for a rename operation.
type RenameRequest struct {
	FullPath    string
	FullNewPath string
	Flags       uint32
}

// RenameReply is returned on a successful rename.
type RenameReply struct{}

// ReadlinkRequest represents a request to read a symbolic link.
type ReadlinkRequest struct {
	FullPath string
}

// ReadlinkReply contains the target of a symbolic link.
type ReadlinkReply struct {
	LinkTarget string
}

// LinkRequest represents a hard link creation request.
type LinkRequest struct {
	OldFullPath string
	NewFullPath string
}

// LinkReply is returned on a successful link call.
type LinkReply struct{}

// SymlinkRequest contains paths for creating a symbolic link.
type SymlinkRequest struct {
	OldFullPath string
	NewFullPath string
}

// SymlinkReply is returned on a successful symlink call.
type SymlinkReply struct{}

// Bitmap values used by SetAttrRequest.ValidAttrs.
const (
	// SATTR_ATIME indicates ATime should be updated.
	SATTR_ATIME = (1 << 0)
	// SATTR_GID indicates GID should be updated.
	SATTR_GID = (1 << 1)
	// SATTR_MODE indicates Mode should be updated.
	SATTR_MODE = (1 << 2)
	// SATTR_MTIME indicates MTime should be updated.
	SATTR_MTIME = (1 << 3)
	// SATTR_SIZE indicates Size should be updated.
	SATTR_SIZE = (1 << 4)
	// SATTR_UID indicates UID should be updated.
	SATTR_UID = (1 << 5)
)

// UTIME_OMIT is used with utimensat() to leave a timestamp unchanged.
const UTIME_OMIT = ((1 << 30) - 2)

// SetAttrRequest describes which attributes of a file should be changed.
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

// SetAttrReply contains the updated attributes of the file.
type SetAttrReply StatReply

// GetMode returns the Mode if it was requested to be set.
func (r *SetAttrRequest) GetMode() (m uint32, ok bool) {
	if r.ValidAttrs&SATTR_MODE == SATTR_MODE {
		return r.Mode, true
	}
	return 0, false
}

// GetUID returns the UID if it was requested to be set.
func (r *SetAttrRequest) GetUID() (u uint32, ok bool) {
	if r.ValidAttrs&SATTR_UID == SATTR_UID {
		return r.UID, true
	}
	return 0, false
}

// GetGID returns the GID if it was requested to be set.
func (r *SetAttrRequest) GetGID() (g uint32, ok bool) {
	if r.ValidAttrs&SATTR_GID == SATTR_GID {
		return r.GID, true
	}
	return 0, false
}

// GetATime returns the access time if it was requested to be set.
func (r *SetAttrRequest) GetATime() (m time.Time, ok bool) {
	if r.ValidAttrs&SATTR_ATIME == SATTR_ATIME {
		return r.ATime, true
	}
	return time.Time{}, false
}

// GetMTime returns the modification time if it was requested to be set.
func (r *SetAttrRequest) GetMTime() (m time.Time, ok bool) {
	if r.ValidAttrs&SATTR_MTIME == SATTR_MTIME {
		return r.MTime, true
	}
	return time.Time{}, false
}

// GetSize returns the Size if it was requested to be set.
func (r *SetAttrRequest) GetSize() (m uint64, ok bool) {
	if r.ValidAttrs&SATTR_SIZE == SATTR_SIZE {
		return r.Size, true
	}
	return 0, false
}

// SetMode marks Mode as valid and sets it.
func (r *SetAttrRequest) SetMode(m uint32) { r.Mode = m; r.ValidAttrs |= SATTR_MODE }

// SetUID marks UID as valid and sets it.
func (r *SetAttrRequest) SetUID(u uint32) { r.UID = u; r.ValidAttrs |= SATTR_UID }

// SetGID marks GID as valid and sets it.
func (r *SetAttrRequest) SetGID(g uint32) { r.GID = g; r.ValidAttrs |= SATTR_GID }

// SetATime marks ATime as valid and sets it.
func (r *SetAttrRequest) SetATime(a time.Time) { r.ATime = a; r.ValidAttrs |= SATTR_ATIME }

// SetMTime marks MTime as valid and sets it.
func (r *SetAttrRequest) SetMTime(m time.Time) { r.MTime = m; r.ValidAttrs |= SATTR_MTIME }

// SetSize marks Size as valid and sets it.
func (r *SetAttrRequest) SetSize(s uint64) { r.Size = s; r.ValidAttrs |= SATTR_SIZE }
