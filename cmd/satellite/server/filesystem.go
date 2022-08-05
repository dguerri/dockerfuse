package server

import (
	"io"
	"os"
)

var dfFS fileSystem = &osFS{}

type fileSystem interface {
	Lstat(name string) (os.FileInfo, error)
	OpenFile(name string, flag int, perm os.FileMode) (file, error)
	ReadDir(name string) ([]os.DirEntry, error)
	Readlink(name string) (string, error)
}

type file interface {
	io.Closer
	io.Reader
	io.ReaderAt
	io.Seeker
	Stat() (os.FileInfo, error)
	Fd() uintptr
	Sync() error
	WriteAt(b []byte, off int64) (n int, err error)
	Write(b []byte) (n int, err error)
}

// osFS implements fileSystem using the local disk
type osFS struct{}

func (*osFS) Lstat(n string) (os.FileInfo, error)                   { return os.Lstat(n) }
func (*osFS) ReadDir(n string) ([]os.DirEntry, error)               { return os.ReadDir(n) }
func (*osFS) Readlink(n string) (string, error)                     { return os.Readlink(n) }
func (*osFS) OpenFile(n string, f int, p os.FileMode) (file, error) { return os.OpenFile(n, f, p) }
