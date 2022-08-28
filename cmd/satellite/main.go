package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/rpc"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dguerri/dockerfuse/cmd/satellite/server"
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
	var persistentLog bool
	flag.BoolVar(&persistentLog, "log", false, "Enable persistent debug log in /tmp/log.txt")
	flag.BoolVar(&printVersion, "version", false, "Print the version and exit")
	flag.Parse()

	if printVersion {
		fmt.Printf("DockerFuse Satellite\nVersion: %s\nGit commmit: %s\n", Version, GitCommit)
		os.Exit(0)
	}

	if persistentLog {
		f, err := os.OpenFile("/tmp/log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0640)
		if err != nil {
			log.Fatalf("error opening log file: %v", err)
		}
		defer f.Close()
		os.Stderr = f
		log.SetOutput(f)
	}

	log.Printf("(%v) Starting up", time.Now())

	fsops := server.NewDockerFuseFSOps()
	log.Printf("setting up signal handler...")
	osSignalChannel := make(chan os.Signal, 1)
	signal.Notify(osSignalChannel, syscall.SIGTERM, syscall.SIGINT)
	go shutdown(fsops, osSignalChannel)
	defer close(osSignalChannel)

	s := rpc.NewServer()
	s.Register(fsops)

	rwCloser := RWCloser{os.Stdin, os.Stdout}
	defer rwCloser.Close()

	log.Printf("Serving requests")
	s.ServeConn(rwCloser)
}

func shutdown(server *server.DockerFuseFSOps, signals <-chan os.Signal) {
	<-signals
	log.Printf("cleaning up...")
	server.CloseAllFDs()

	os.Exit(0)
}
