package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	daemon "github.com/sevlyar/go-daemon"

	"github.com/dguerri/dockerfuse/cmd/dockerfuse/client"
)

const (
	attrTTL  = 1500 * time.Millisecond
	entryTTL = 1500 * time.Millisecond
)

var (
	containerID  string
	mountPoint   string
	daemonize    bool
	printVersion bool
	// Version holds the version tag, and it is set at build-time
	Version string
	// GitCommit holds the git commit used to build the binary. It is set at build-time
	GitCommit string
)

func init() {
	flag.BoolVar(&printVersion, "version", false, "Print the version and exit")
	flag.BoolVar(&printVersion, "v", false, "Print the version and exit")

	flag.StringVar(&containerID, "id", "", "Docker containter ID (or name)")
	flag.StringVar(&containerID, "i", "", "Docker containter ID (or name)")

	flag.StringVar(&mountPoint, "mount", "", "Mount point for containter FS")
	flag.StringVar(&mountPoint, "m", "", "Mount point for containter FS")

	flag.BoolVar(&daemonize, "daemonize", false, "Daemonize fuse process")
	flag.BoolVar(&daemonize, "d", false, "Daemonize fuse process")

}

func main() {
	flag.Parse()
	if printVersion {
		fmt.Printf("DockerFuse\nVersion: %s\nGit commmit: %s\n", Version, GitCommit)
		os.Exit(0)
	}

	if containerID == "" {
		log.Printf("container id is not specified.\n")
		flag.Usage()
		os.Exit(2)
	}
	if mountPoint == "" {
		log.Printf("mount point is not specified.\n")
		flag.Usage()
		os.Exit(2)
	}

	if daemonize {
		ctx := daemon.Context{}
		child, err := ctx.Reborn()
		if err != nil {
			log.Printf("daemonization failed: %s", err)
			os.Exit(3)
		}
		if child != nil {
			// parent process
			return
		}
	}

	log.Printf("checking if mount directory exists (%v)...", mountPoint)
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		os.Exit(4)
	}

	user, err := user.Current()
	if err != nil {
		log.Fatalf("cannot get current user: %s", err)
	}
	uid, err := strconv.Atoi(user.Uid)
	if err != nil {
		log.Fatalf("invalid uid (%s): %s", user.Uid, err)
	}
	gid, err := strconv.Atoi(user.Gid)
	if err != nil {
		log.Fatalf("invalid gid (%s): %s", user.Gid, err)
	}

	fuseDockerClient, err := client.NewDockerFuseClient(containerID)
	if err != nil {
		log.Panicf("error initiazializing docker client: %s", err)
	} else {
		log.Printf("docker client created")
	}

	log.Printf("mounting FS to %v...", mountPoint)
	vEntryTTL := entryTTL
	vAttrTTL := attrTTL
	server, err := fs.Mount(mountPoint, client.NewNode(fuseDockerClient, "/", ""), &fs.Options{
		EntryTimeout:    &vEntryTTL,
		AttrTimeout:     &vAttrTTL,
		NegativeTimeout: &vEntryTTL,
		MountOptions: fuse.MountOptions{
			FsName: fmt.Sprintf("dockerfuse-%s", containerID),
		},
		UID: uint32(uid),
		GID: uint32(gid),
	})
	if err != nil {
		log.Printf("mount failed: %s", err)
		os.Exit(5)
	}

	log.Printf("setting up signal handler...")
	osSignalChannel := make(chan os.Signal, 1)
	signal.Notify(osSignalChannel, syscall.SIGTERM, syscall.SIGINT)
	go shutdown(server, osSignalChannel)
	defer close(osSignalChannel)

	server.Wait()
}

func shutdown(server *fuse.Server, signals <-chan os.Signal) {
	<-signals
	if err := server.Unmount(); err != nil {
		log.Printf("server unmount failed: %v", err)
		os.Exit(1)
	}

	log.Printf("unmount successful")
	os.Exit(0)
}
