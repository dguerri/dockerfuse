package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/rpc"
	"os"
	"time"

	"dockerfuse/cmd/satellite/server"
)

// rwCloser just merges a ReadCloser and a WriteCloser into a ReadWriteCloser.
type RWCloser struct {
	io.ReadCloser
	io.WriteCloser
}

func (rw RWCloser) Close() error {
	err := rw.ReadCloser.Close()
	if err := rw.WriteCloser.Close(); err != nil {
		return err
	}
	return err
}

// Version and GitCommit are set at build-time
var (
	Version   string
	GitCommit string
)

func main() {
	var printVersion bool
	flag.BoolVar(&printVersion, "version", false, "Print the version and exit")
	flag.Parse()

	if printVersion {
		fmt.Printf("DockerFuse Satellite\nVersion: %s\nGit commmit: %s\n", Version, GitCommit)
		os.Exit(0)
	}

	f, err := os.OpenFile("/tmp/log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	os.Stderr = f
	log.SetOutput(f)
	log.Printf("(%v) Starting up", time.Now())

	fsops := server.NewDockerFuseFSOps()
	s := rpc.NewServer()
	s.Register(fsops)

	rwCloser := RWCloser{os.Stdin, os.Stdout}
	defer rwCloser.Close()

	log.Printf("Serving requests")
	s.ServeConn(rwCloser)
}
