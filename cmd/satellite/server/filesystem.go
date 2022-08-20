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
	Remove(name string) error
	Mkdir(name string, perm os.FileMode) error
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

func (*osFS) Lstat(n string) (os.FileInfo, error)                   { return os.Lstat(n) }
func (*osFS) ReadDir(n string) ([]os.DirEntry, error)               { return os.ReadDir(n) }
func (*osFS) Readlink(n string) (string, error)                     { return os.Readlink(n) }
func (*osFS) OpenFile(n string, f int, p os.FileMode) (file, error) { return os.OpenFile(n, f, p) }
func (*osFS) Remove(n string) error                                 { return os.Remove(n) }
func (*osFS) Mkdir(n string, p os.FileMode) error                   { return os.Mkdir(n, p) }
