package main

import (
	"flag"
	"fmt"
	"log/slog"
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

// Exit codes
const (
	errorNone             = 0
	errorArgs             = 1
	errorDaemon           = 2
	errorCreateMount      = 3
	errorGetUser          = 4
	errorInvalidUidGid    = 5
	errorInitDockerClient = 6
	errorMountUnmount     = 7
)

var (
	containerID  string
	mountPoint   string
	path         string
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

	flag.StringVar(&containerID, "id", "", "Docker container ID (or name)")
	flag.StringVar(&containerID, "i", "", "Docker container ID (or name)")

	flag.StringVar(&mountPoint, "mount", "", "Mount point for container FS")
	flag.StringVar(&mountPoint, "m", "", "Mount point for container FS")

	flag.StringVar(&path, "path", "/", "Path inside the container")
	flag.StringVar(&path, "p", "/", "Path inside the container")

	flag.BoolVar(&daemonize, "daemonize", false, "Daemonize fuse process")
	flag.BoolVar(&daemonize, "d", false, "Daemonize fuse process")

}

func main() {
	flag.Parse()
	if printVersion {
		fmt.Printf("DockerFuse\nVersion: %s\nGit commit: %s\n", Version, GitCommit)
		os.Exit(errorNone)
	}

	if containerID == "" {
		slog.Error("container id is not specified.\n")
		flag.Usage()
		os.Exit(errorArgs)
	}
	if mountPoint == "" {
		slog.Error("mount point is not specified.\n")
		flag.Usage()
		os.Exit(errorArgs)
	}

	if daemonize {
		ctx := daemon.Context{}
		child, err := ctx.Reborn()
		if err != nil {
			slog.Error("daemonization failed", "error", err)
			os.Exit(errorDaemon)
		}
		if child != nil {
			// parent process
			return
		}
	}

	slog.Debug("checking if mount directory exists", "path", mountPoint)
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		slog.Error("failed to create mount directory", "path", mountPoint, "error", err)
		os.Exit(errorCreateMount)
	}

	user, err := user.Current()
	if err != nil {
		slog.Error("cannot get current user", "error", err)
		os.Exit(errorGetUser)
	}
	uid, err := strconv.Atoi(user.Uid)
	if err != nil {
		slog.Error("invalid uid", "uid", user.Uid, "error", err)
		os.Exit(errorInvalidUidGid)
	}
	gid, err := strconv.Atoi(user.Gid)
	if err != nil {
		slog.Error("invalid gid", "gid", user.Gid, "error", err)
		os.Exit(errorInvalidUidGid)
	}

	fuseDockerClient, err := client.NewDockerFuseClient(containerID)
	if err != nil {
		slog.Error("error initializing docker client", "error", err)
		os.Exit(errorInitDockerClient)
	} else {
		slog.Debug("docker client created")
	}

	slog.Info("mounting FS", "path", mountPoint)
	vEntryTTL := entryTTL
	vAttrTTL := attrTTL
	server, err := fs.Mount(mountPoint, client.NewNode(fuseDockerClient, path, ""), &fs.Options{
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
		slog.Error("mount failed", "error", err)
		os.Exit(errorMountUnmount)
	}

	slog.Debug("setting up signal handler...")
	osSignalChannel := make(chan os.Signal, 1)
	signal.Notify(osSignalChannel, syscall.SIGTERM, syscall.SIGINT)
	go shutdown(server, osSignalChannel)
	defer close(osSignalChannel)

	server.Wait()
}

func shutdown(server *fuse.Server, signals <-chan os.Signal) {
	<-signals
	if err := server.Unmount(); err != nil {
		slog.Error("unmount failed", "error", err)
		os.Exit(errorMountUnmount)
	}

	slog.Info("unmount successful")
	os.Exit(errorNone)
}
