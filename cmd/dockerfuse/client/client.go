package client

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/dguerri/dockerfuse/pkg/rpccommon"
	"github.com/docker/cli/cli/connhelper"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

const (
	satelliteBinPrefix = "dockerfuse_satellite"
	satelliteExecPath  = "/tmp"
)

type statAttr struct {
	FuseAttr   fuse.Attr
	LinkTarget string
}

// DockerFuseClientInterface can be used to write unit tests
type DockerFuseClientInterface interface {
	disconnect()
	connectSatellite(ctx context.Context) (err error)

	close(ctx context.Context, fh fusefs.FileHandle) (syserr syscall.Errno)
	create(ctx context.Context, fullPath string, flags int, mode fs.FileMode, attr *statAttr) (fh fusefs.FileHandle, syserr syscall.Errno)
	fsync(ctx context.Context, fh fusefs.FileHandle, flags uint32) (syserr syscall.Errno)
	link(ctx context.Context, oldFullPath string, newFullPath string) (syserr syscall.Errno)
	mkdir(ctx context.Context, fullPath string, mode fs.FileMode, attr *statAttr) (syserr syscall.Errno)
	open(ctx context.Context, fullPath string, flags int, modeIn fs.FileMode) (fh fusefs.FileHandle, mode fs.FileMode, syserr syscall.Errno)
	read(ctx context.Context, fh fusefs.FileHandle, offset int64, n int) (data []byte, syserr syscall.Errno)
	readDir(ctx context.Context, fullPath string) (ds fusefs.DirStream, syserr syscall.Errno)
	readlink(ctx context.Context, fullPath string) (linkTarget []byte, syserr syscall.Errno)
	rename(ctx context.Context, fullPath string, fullNewPath string, flags uint32) (syserr syscall.Errno)
	rmdir(ctx context.Context, fullPath string) (syserr syscall.Errno)
	seek(ctx context.Context, fh fusefs.FileHandle, offset int64, whence int) (n int64, syserr syscall.Errno)
	setAttr(ctx context.Context, fullPath string, in *fuse.SetAttrIn, out *statAttr) (syserr syscall.Errno)
	stat(ctx context.Context, fullPath string, attr *statAttr) (syserr syscall.Errno)
	symlink(ctx context.Context, oldFullPath string, newFullPath string) (syserr syscall.Errno)
	unlink(ctx context.Context, fullPath string) (syserr syscall.Errno)
	write(ctx context.Context, fh fusefs.FileHandle, offset int64, data []byte) (n int, syserr syscall.Errno)
}

// DockerFuseClient is used to communicate with the Docker API server
type DockerFuseClient struct {
	dockerClient            dockerClient
	rpcClient               rpcClient
	containerID             string
	satelliteFullRemotePath string
}

// NewDockerFuseClient returns a new DockerFuseClient pointer
func NewDockerFuseClient(containerID string) (*DockerFuseClient, error) {
	var clientOpts []client.Opt = nil
	if strings.HasPrefix(os.Getenv("DOCKER_HOST"), "ssh://") {
		helper, err := connhelper.GetConnectionHelper(os.Getenv("DOCKER_HOST"))
		if err != nil {
			return nil, err
		}

		httpClient := &http.Client{
			Transport: &http.Transport{
				DialContext: helper.Dialer,
			},
		}

		clientOpts = append(clientOpts,
			client.WithHTTPClient(httpClient),
			client.WithHost(helper.Host),
			client.WithDialContext(helper.Dialer),
		)
	} else {
		clientOpts = append(clientOpts, client.FromEnv)
	}
	version := os.Getenv("DOCKER_API_VERSION")
	if version != "" {
		clientOpts = append(clientOpts, client.WithVersion(version))
	} else {
		clientOpts = append(clientOpts, client.WithAPIVersionNegotiation())
	}
	docker, err := dockerCF.NewClientWithOpts(clientOpts...)
	if err != nil {
		return nil, err
	}
	fdc := &DockerFuseClient{
		dockerClient: docker,
		containerID:  containerID,
	}

	ctx := context.Background()
	err = fdc.uploadSatellite(ctx)
	if err != nil {
		return nil, fmt.Errorf("error copying docker-fuse satellite to remote container: %s", err)
	}
	err = fdc.connectSatellite(ctx)
	if err != nil {
		return nil, fmt.Errorf("error connecting to docker-fuse satellite: %s", err)
	}
	return fdc, nil
}

func (d *DockerFuseClient) disconnect() {
	if d.rpcClient != nil {
		d.rpcClient.Close()
		d.rpcClient = nil
	}
}

func (d *DockerFuseClient) uploadSatellite(ctx context.Context) (err error) {
	containerInspect, err := d.dockerClient.ContainerInspect(ctx, d.containerID)
	if err != nil {
		return err
	}

	imageInspect, _, err := d.dockerClient.ImageInspectWithRaw(ctx, containerInspect.Image)
	if err != nil {
		return err
	}

	if imageInspect.Architecture != "arm64" && imageInspect.Architecture != "amd64" {
		return fmt.Errorf("unsupported architecture: %s (use arm64 or amd64)", imageInspect.Architecture)
	}

	satelliteBinName := fmt.Sprintf("%s_%s", satelliteBinPrefix, imageInspect.Architecture)
	d.satelliteFullRemotePath = filepath.Clean(filepath.Join(satelliteExecPath, satelliteBinName))

	ex, err := dfFS.Executable()
	if err != nil {
		return err
	}
	satelliteFullLocalPath := filepath.Clean(filepath.Join(filepath.Dir(ex), satelliteBinName))
	satelliteBin, err := dfFS.ReadFile(satelliteFullLocalPath)
	if err != nil {
		return err
	}

	// Create tar archive in memory
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	err = tw.WriteHeader(&tar.Header{
		Name: satelliteBinName,
		Mode: 0700,
		Size: int64(len(satelliteBin)),
	})
	if err != nil {
		return err
	}
	tw.Write([]byte(satelliteBin))
	tw.Close()

	slog.Info("copying", "source", satelliteFullLocalPath, "destination", fmt.Sprintf("%s:%s", d.containerID, d.satelliteFullRemotePath))
	tr := bufio.NewReader(&buf)
	err = d.dockerClient.CopyToContainer(ctx, d.containerID, satelliteExecPath, tr, container.CopyToContainerOptions{})
	if err != nil {
		return err
	}

	return
}

func (d *DockerFuseClient) connectSatellite(ctx context.Context) (err error) {
	if d.rpcClient != nil {
		// Reconnect
		d.disconnect()
	}

	config := container.ExecOptions{
		AttachStderr: false,
		AttachStdout: true,
		AttachStdin:  true,
		Tty:          false,
		Cmd:          []string{d.satelliteFullRemotePath},
	}
	execID, err := d.dockerClient.ContainerExecCreate(ctx, d.containerID, config)
	if err != nil {
		return
	}
	hl, err := d.dockerClient.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{Tty: true})
	if err != nil {
		return
	}
	d.rpcClient = rpcCF.NewClient(hl.Conn)
	return
}

func (d *DockerFuseClient) stat(ctx context.Context, fullPath string, attr *statAttr) (syserr syscall.Errno) {
	var (
		reply   rpccommon.StatReply
		request rpccommon.StatRequest
	)
	request.FullPath = fullPath

	err := d.rpcClient.Call("DockerFuseFSOps.Stat", request, &reply)
	if err != nil {
		syserr = rpccommon.RPCErrorStringTOErrno(err)
		return
	}

	attr.FuseAttr.Ino = reply.Ino
	attr.FuseAttr.Size = uint64(reply.Size)
	attr.FuseAttr.Blocks = uint64(reply.Blocks)
	attr.FuseAttr.Atime = uint64(reply.Atime)
	attr.FuseAttr.Mtime = uint64(reply.Mtime)
	attr.FuseAttr.Ctime = uint64(reply.Ctime)
	attr.FuseAttr.Mode = reply.Mode
	attr.FuseAttr.Nlink = reply.Nlink
	attr.FuseAttr.Owner.Uid = reply.UID
	attr.FuseAttr.Owner.Gid = reply.GID
	attr.LinkTarget = reply.LinkTarget
	return
}

func (d *DockerFuseClient) create(ctx context.Context, fullPath string, flags int, mode fs.FileMode, attr *statAttr) (fh fusefs.FileHandle, syserr syscall.Errno) {
	var reply rpccommon.OpenReply

	request := rpccommon.OpenRequest{
		FullPath: fullPath,
		SAFlags:  rpccommon.SystemToSAFlags(flags),
		Mode:     mode,
	}
	err := d.rpcClient.Call("DockerFuseFSOps.Open", request, &reply)
	if err != nil {
		syserr = rpccommon.RPCErrorStringTOErrno(err)
		return
	}

	fh = reply.FD
	attr.FuseAttr.Ino = reply.Ino
	attr.FuseAttr.Size = uint64(reply.Size)
	attr.FuseAttr.Blocks = uint64(reply.Blocks)
	attr.FuseAttr.Atime = uint64(reply.Atime)
	attr.FuseAttr.Mtime = uint64(reply.Mtime)
	attr.FuseAttr.Ctime = uint64(reply.Ctime)
	attr.FuseAttr.Mode = reply.Mode
	attr.FuseAttr.Nlink = reply.Nlink
	attr.FuseAttr.Owner.Uid = reply.UID
	attr.FuseAttr.Owner.Gid = reply.GID
	attr.LinkTarget = reply.LinkTarget
	return
}

func (d *DockerFuseClient) readDir(ctx context.Context, fullPath string) (ds fusefs.DirStream, syserr syscall.Errno) {
	var reply rpccommon.ReadDirReply

	err := d.rpcClient.Call("DockerFuseFSOps.ReadDir", rpccommon.StatRequest{FullPath: fullPath}, &reply)
	if err != nil {
		syserr = rpccommon.RPCErrorStringTOErrno(err)
		return
	}

	dirEntries := make([]fuse.DirEntry, 0, len(reply.DirEntries))
	for _, entry := range reply.DirEntries {
		if entry.Ino > 2 {
			dirEntries = append(dirEntries, fuse.DirEntry{
				Mode: uint32(entry.Mode),
				Ino:  uint64(entry.Ino),
				Name: entry.Name,
			})
		}
	}
	ds = fusefs.NewListDirStream(dirEntries)

	return
}

func (d *DockerFuseClient) open(ctx context.Context, fullPath string, flags int, modeIn fs.FileMode) (fh fusefs.FileHandle, mode fs.FileMode, syserr syscall.Errno) {
	var reply rpccommon.OpenReply

	request := rpccommon.OpenRequest{
		FullPath: fullPath,
		SAFlags:  rpccommon.SystemToSAFlags(flags),
		Mode:     modeIn,
	}
	err := d.rpcClient.Call("DockerFuseFSOps.Open", request, &reply)
	if err != nil {
		syserr = rpccommon.RPCErrorStringTOErrno(err)
		return
	}

	fh = reply.FD
	mode = os.FileMode(reply.Mode)
	return
}

func (d *DockerFuseClient) close(ctx context.Context, fh fusefs.FileHandle) (syserr syscall.Errno) {
	var reply rpccommon.CloseReply

	err := d.rpcClient.Call("DockerFuseFSOps.Close", rpccommon.CloseRequest{FD: fh.(uintptr)}, &reply)
	if err != nil {
		syserr = rpccommon.RPCErrorStringTOErrno(err)
		return
	}

	return
}

func (d *DockerFuseClient) read(ctx context.Context, fh fusefs.FileHandle, offset int64, n int) (data []byte, syserr syscall.Errno) {
	var reply rpccommon.ReadReply

	err := d.rpcClient.Call("DockerFuseFSOps.Read", rpccommon.ReadRequest{FD: fh.(uintptr), Offset: offset, Num: n}, &reply)
	if err != nil {
		if err.Error() == "EOF" {
			data = make([]byte, 0)
			return data, 0
		}
		syserr = rpccommon.RPCErrorStringTOErrno(err)
		return nil, syserr
	}

	data = reply.Data
	return data, 0
}

func (d *DockerFuseClient) seek(ctx context.Context, fh fusefs.FileHandle, offset int64, whence int) (n int64, syserr syscall.Errno) {
	var reply rpccommon.SeekReply

	err := d.rpcClient.Call("DockerFuseFSOps.Seek", rpccommon.SeekRequest{FD: fh.(uintptr), Offset: offset, Whence: whence}, &reply)
	if err != nil {
		syserr = rpccommon.RPCErrorStringTOErrno(err)
		return
	}

	n = reply.Num
	return
}

func (d *DockerFuseClient) write(ctx context.Context, fh fusefs.FileHandle, offset int64, data []byte) (n int, syserr syscall.Errno) {
	var reply rpccommon.WriteReply

	err := d.rpcClient.Call("DockerFuseFSOps.Write", rpccommon.WriteRequest{FD: fh.(uintptr), Offset: offset, Data: data}, &reply)
	if err != nil {
		syserr = rpccommon.RPCErrorStringTOErrno(err)
		return
	}

	n = reply.Num
	return
}

func (d *DockerFuseClient) unlink(ctx context.Context, fullPath string) (syserr syscall.Errno) {
	var reply rpccommon.UnlinkReply

	err := d.rpcClient.Call("DockerFuseFSOps.Unlink", rpccommon.UnlinkRequest{FullPath: fullPath}, &reply)
	if err != nil {
		return rpccommon.RPCErrorStringTOErrno(err)
	}
	return
}

func (d *DockerFuseClient) fsync(ctx context.Context, fh fusefs.FileHandle, flags uint32) (syserr syscall.Errno) {
	var reply rpccommon.FsyncReply

	err := d.rpcClient.Call("DockerFuseFSOps.Fsync", rpccommon.FsyncRequest{FD: fh.(uintptr), Flags: flags}, &reply)
	if err != nil {
		return rpccommon.RPCErrorStringTOErrno(err)
	}
	return
}

func (d *DockerFuseClient) mkdir(ctx context.Context, fullPath string, mode fs.FileMode, attr *statAttr) (syserr syscall.Errno) {
	var reply rpccommon.MkdirReply

	request := rpccommon.MkdirRequest{
		FullPath: fullPath,
		Mode:     mode,
	}
	err := d.rpcClient.Call("DockerFuseFSOps.Mkdir", request, &reply)
	if err != nil {
		return rpccommon.RPCErrorStringTOErrno(err)
	}
	attr.FuseAttr.Ino = reply.Ino
	attr.FuseAttr.Size = uint64(reply.Size)
	attr.FuseAttr.Blocks = uint64(reply.Blocks)
	attr.FuseAttr.Atime = uint64(reply.Atime)
	attr.FuseAttr.Mtime = uint64(reply.Mtime)
	attr.FuseAttr.Ctime = uint64(reply.Ctime)
	attr.FuseAttr.Mode = reply.Mode
	attr.FuseAttr.Nlink = reply.Nlink
	attr.FuseAttr.Owner.Uid = reply.UID
	attr.FuseAttr.Owner.Gid = reply.GID
	attr.LinkTarget = ""
	return
}

func (d *DockerFuseClient) rmdir(ctx context.Context, fullPath string) (syserr syscall.Errno) {
	var reply rpccommon.RmdirReply

	request := rpccommon.RmdirRequest{FullPath: fullPath}
	err := d.rpcClient.Call("DockerFuseFSOps.Rmdir", request, &reply)
	if err != nil {
		return rpccommon.RPCErrorStringTOErrno(err)
	}
	return
}

func (d *DockerFuseClient) rename(ctx context.Context, fullPath string, fullNewPath string, flags uint32) (syserr syscall.Errno) {
	var reply rpccommon.RenameReply

	request := rpccommon.RenameRequest{FullPath: fullPath, FullNewPath: fullNewPath, Flags: flags}
	err := d.rpcClient.Call("DockerFuseFSOps.Rename", request, &reply)
	if err != nil {
		return rpccommon.RPCErrorStringTOErrno(err)
	}
	return
}

func (d *DockerFuseClient) readlink(ctx context.Context, fullPath string) (linkTarget []byte, syserr syscall.Errno) {
	var reply rpccommon.ReadlinkReply

	request := rpccommon.ReadlinkRequest{FullPath: fullPath}
	err := d.rpcClient.Call("DockerFuseFSOps.Readlink", request, &reply)
	if err != nil {
		return []byte{}, rpccommon.RPCErrorStringTOErrno(err)
	}
	return []byte(reply.LinkTarget), 0
}

func (d *DockerFuseClient) link(ctx context.Context, oldFullPath string, newFullPath string) (syserr syscall.Errno) {
	var reply rpccommon.LinkReply

	request := rpccommon.LinkRequest{OldFullPath: oldFullPath, NewFullPath: newFullPath}
	err := d.rpcClient.Call("DockerFuseFSOps.Link", request, &reply)
	if err != nil {
		return rpccommon.RPCErrorStringTOErrno(err)
	}
	return 0
}

func (d *DockerFuseClient) symlink(ctx context.Context, oldFullPath string, newFullPath string) (syserr syscall.Errno) {
	var reply rpccommon.SymlinkReply

	request := rpccommon.SymlinkRequest{OldFullPath: oldFullPath, NewFullPath: newFullPath}
	err := d.rpcClient.Call("DockerFuseFSOps.Symlink", request, &reply)
	if err != nil {
		return rpccommon.RPCErrorStringTOErrno(err)
	}
	return 0
}

func (d *DockerFuseClient) setAttr(ctx context.Context, fullPath string, in *fuse.SetAttrIn, out *statAttr) (syserr syscall.Errno) {
	var (
		request rpccommon.SetAttrRequest
		reply   rpccommon.SetAttrReply
	)

	request = rpccommon.SetAttrRequest{FullPath: fullPath}
	if atime, ok := in.GetATime(); ok {
		request.SetATime(atime)
	}
	if mtime, ok := in.GetMTime(); ok {
		request.SetMTime(mtime)
	}
	if uid, ok := in.GetUID(); ok {
		request.SetUID(uid)
	}
	if gid, ok := in.GetGID(); ok {
		request.SetGID(gid)
	}
	if mode, ok := in.GetMode(); ok {
		request.SetMode(mode)
	}
	if size, ok := in.GetSize(); ok {
		request.SetSize(size)
	}

	err := d.rpcClient.Call("DockerFuseFSOps.SetAttr", request, &reply)
	if err != nil {
		return rpccommon.RPCErrorStringTOErrno(err)
	}

	out.FuseAttr.Ino = reply.Ino
	out.FuseAttr.Size = uint64(reply.Size)
	out.FuseAttr.Blocks = uint64(reply.Blocks)
	out.FuseAttr.Atime = uint64(reply.Atime)
	out.FuseAttr.Mtime = uint64(reply.Mtime)
	out.FuseAttr.Ctime = uint64(reply.Ctime)
	out.FuseAttr.Mode = reply.Mode
	out.FuseAttr.Nlink = reply.Nlink
	out.FuseAttr.Owner.Uid = reply.UID
	out.FuseAttr.Owner.Gid = reply.GID
	out.LinkTarget = reply.LinkTarget
	return 0
}
