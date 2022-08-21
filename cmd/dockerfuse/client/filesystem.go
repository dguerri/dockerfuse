package client

import "os"

var dfFS fileSystem = &osFS{}

type fileSystem interface {
	Executable() (string, error)
	ReadFile(name string) ([]byte, error)
}

// osFS implements fileSystem using the local disk
type osFS struct{}

func (*osFS) Executable() (string, error)       { return os.Executable() }
func (*osFS) ReadFile(n string) ([]byte, error) { return os.ReadFile(n) }
