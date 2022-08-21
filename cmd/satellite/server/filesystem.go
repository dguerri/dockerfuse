package server

import (
	"io"
	"os"
	"syscall"
)

var dfFS fileSystem = &osFS{}

type fileSystem interface {
	Chmod(name string, mode os.FileMode) error
	Chown(name string, uid, gid int) error
	Link(oldname, newname string) error
	Lstat(name string) (os.FileInfo, error)
	Mkdir(name string, perm os.FileMode) error
	OpenFile(name string, flag int, perm os.FileMode) (file, error)
	ReadDir(name string) ([]os.DirEntry, error)
	Readlink(name string) (string, error)
	Remove(name string) error
	Rename(oldpath, newpath string) error
	Symlink(oldname, newname string) error
	Truncate(name string, size int64) error

	UtimesNano(path string, ts []syscall.Timespec) error // From syscall (not os)
}

type file interface {
	Fd() uintptr
	io.Closer
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Writer
	io.WriterAt
	Stat() (os.FileInfo, error)
	Sync() error
}

// osFS implements fileSystem using the local disk
type osFS struct{}

func (*osFS) Chmod(n string, m os.FileMode) error                   { return os.Chmod(n, m) }
func (*osFS) Chown(n string, u, g int) error                        { return os.Chown(n, u, g) }
func (*osFS) Link(o, n string) error                                { return os.Link(o, n) }
func (*osFS) Lstat(n string) (os.FileInfo, error)                   { return os.Lstat(n) }
func (*osFS) Mkdir(n string, p os.FileMode) error                   { return os.Mkdir(n, p) }
func (*osFS) OpenFile(n string, f int, p os.FileMode) (file, error) { return os.OpenFile(n, f, p) }
func (*osFS) ReadDir(n string) ([]os.DirEntry, error)               { return os.ReadDir(n) }
func (*osFS) Readlink(n string) (string, error)                     { return os.Readlink(n) }
func (*osFS) Remove(n string) error                                 { return os.Remove(n) }
func (*osFS) Rename(o, n string) error                              { return os.Rename(o, n) }
func (*osFS) Symlink(o, n string) error                             { return os.Symlink(o, n) }
func (*osFS) Truncate(n string, s int64) error                      { return os.Truncate(n, s) }

// We need the following from syscall as os.Chtimes doesn't support leaving timestamps unchanged
func (*osFS) UtimesNano(p string, t []syscall.Timespec) error { return syscall.UtimesNano(p, t) }
