package client

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"dockerfuse/pkg/rpc_common"
)

const (
	satelliteBinPrefix = "dockerfuse_satellite"
	satelliteExecPath  = "/tmp"
)

type StatAttr struct {
	FuseAttr   fuse.Attr
	LinkTarget string
}
type FuseDockerClientInterface interface {
	isConnected() bool
	disconnect()
	connectSatellite(ctx context.Context) (err error)

	close(ctx context.Context, fh fusefs.FileHandle) (syserr syscall.Errno)
	create(ctx context.Context, fullPath string, flags int, mode fs.FileMode, attr *StatAttr) (fh fusefs.FileHandle, syserr syscall.Errno)
	fsync(ctx context.Context, fh fusefs.FileHandle, flags uint32) (syserr syscall.Errno)
	link(ctx context.Context, oldFullPath string, newFullPath string) (syserr syscall.Errno)
	mkdir(ctx context.Context, fullPath string, mode fs.FileMode, attr *StatAttr) (syserr syscall.Errno)
	open(ctx context.Context, fullPath string, flags int, mode_in fs.FileMode) (fh fusefs.FileHandle, mode fs.FileMode, syserr syscall.Errno)
	read(ctx context.Context, fh fusefs.FileHandle, offset int64, n int) (data []byte, syserr syscall.Errno)
	readDir(ctx context.Context, fullPath string) (ds fusefs.DirStream, syserr syscall.Errno)
	readlink(ctx context.Context, fullPath string) (linkTarget []byte, syserr syscall.Errno)
	rename(ctx context.Context, fullPath string, fullNewPath string, flags uint32) (syserr syscall.Errno)
	rmdir(ctx context.Context, fullPath string) (syserr syscall.Errno)
	seek(ctx context.Context, fh fusefs.FileHandle, offset int64, whence int) (n int64, syserr syscall.Errno)
	setAttr(ctx context.Context, fullPath string, in *fuse.SetAttrIn, out *StatAttr) (syserr syscall.Errno)
	stat(ctx context.Context, fullPath string, fh fusefs.FileHandle, attr *StatAttr) (syserr syscall.Errno)
	symlink(ctx context.Context, oldFullPath string, newFullPath string) (syserr syscall.Errno)
	unlink(ctx context.Context, fullPath string) (syserr syscall.Errno)
	write(ctx context.Context, fh fusefs.FileHandle, offset int64, data []byte) (n int, syserr syscall.Errno)
}

type FuseDockerClient struct {
	dockerClient            dockerClient
	rpcClient               rpcClient
	containerID             string
	satelliteFullRemotePath string
}

func NewFuseDockerClient(containerID string) (*FuseDockerClient, error) {
	docker, err := dockerCF.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	fdc := &FuseDockerClient{
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

func (d *FuseDockerClient) isConnected() bool {
	return d.rpcClient != nil
}

func (d *FuseDockerClient) disconnect() {
	if d.rpcClient != nil {
		d.rpcClient.Close()
		d.rpcClient = nil
	}
}

func (d *FuseDockerClient) uploadSatellite(ctx context.Context) (err error) {
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

	ex, err := os.Executable()
	if err != nil {
		return err
	}
	satelliteFullLocalPath := filepath.Clean(filepath.Join(filepath.Dir(ex), satelliteBinName))
	satelliteBin, err := os.ReadFile(satelliteFullLocalPath)
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

	log.Printf("copying %s to %s:%s", satelliteFullLocalPath, d.containerID, d.satelliteFullRemotePath)
	tr := bufio.NewReader(&buf)
	err = d.dockerClient.CopyToContainer(ctx, d.containerID, satelliteExecPath, tr, types.CopyToContainerOptions{})
	if err != nil {
		return err
	}

	return
}

func (d *FuseDockerClient) connectSatellite(ctx context.Context) (err error) {
	if d.rpcClient != nil {
		// Reconnect
		d.disconnect()
	}

	config := types.ExecConfig{
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
	hl, err := d.dockerClient.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{Tty: true})
	if err != nil {
		return
	}
	d.rpcClient = rpcCF.NewClient(hl.Conn)
	return
}

func (d *FuseDockerClient) stat(ctx context.Context, fullPath string, fh fusefs.FileHandle, attr *StatAttr) (syserr syscall.Errno) {
	var (
		reply   rpc_common.StatReply
		request rpc_common.StatRequest
	)

	request.FullPath = fullPath
	if fh != nil {
		request.FD = fh.(uintptr)
		request.UseFD = true
	}

	err := d.rpcClient.Call("DockerFuseFSOps.Stat", request, &reply)
	if err != nil {
		syserr = rpc_common.RPCErrorStringTOErrno(err)
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
	attr.FuseAttr.Owner.Uid = reply.Uid
	attr.FuseAttr.Owner.Gid = reply.Gid
	attr.LinkTarget = reply.LinkTarget
	return
}

func (d *FuseDockerClient) create(ctx context.Context, fullPath string, flags int, mode fs.FileMode, attr *StatAttr) (fh fusefs.FileHandle, syserr syscall.Errno) {
	var reply rpc_common.OpenReply

	request := rpc_common.OpenRequest{
		FullPath: fullPath,
		SAFlags:  rpc_common.SystemToSAFlags(flags),
		Mode:     mode,
	}
	err := d.rpcClient.Call("DockerFuseFSOps.Open", request, &reply)
	if err != nil {
		syserr = rpc_common.RPCErrorStringTOErrno(err)
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
	attr.FuseAttr.Owner.Uid = reply.Uid
	attr.FuseAttr.Owner.Gid = reply.Gid
	attr.LinkTarget = reply.LinkTarget
	return
}

func (d *FuseDockerClient) readDir(ctx context.Context, fullPath string) (ds fusefs.DirStream, syserr syscall.Errno) {
	var reply rpc_common.ReadDirReply

	err := d.rpcClient.Call("DockerFuseFSOps.ReadDir", rpc_common.StatRequest{FullPath: fullPath}, &reply)
	if err != nil {
		syserr = rpc_common.RPCErrorStringTOErrno(err)
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

func (d *FuseDockerClient) open(ctx context.Context, fullPath string, flags int, mode_in fs.FileMode) (fh fusefs.FileHandle, mode fs.FileMode, syserr syscall.Errno) {
	var reply rpc_common.OpenReply

	request := rpc_common.OpenRequest{
		FullPath: fullPath,
		SAFlags:  rpc_common.SystemToSAFlags(flags),
		Mode:     mode_in,
	}
	err := d.rpcClient.Call("DockerFuseFSOps.Open", request, &reply)
	if err != nil {
		syserr = rpc_common.RPCErrorStringTOErrno(err)
		return
	}

	fh = reply.FD
	mode = os.FileMode(reply.Mode)
	return
}

func (d *FuseDockerClient) close(ctx context.Context, fh fusefs.FileHandle) (syserr syscall.Errno) {
	var reply rpc_common.CloseReply

	err := d.rpcClient.Call("DockerFuseFSOps.Close", rpc_common.CloseRequest{FD: fh.(uintptr)}, &reply)
	if err != nil {
		syserr = rpc_common.RPCErrorStringTOErrno(err)
		return
	}

	return
}

func (d *FuseDockerClient) read(ctx context.Context, fh fusefs.FileHandle, offset int64, n int) (data []byte, syserr syscall.Errno) {
	var reply rpc_common.ReadReply

	err := d.rpcClient.Call("DockerFuseFSOps.Read", rpc_common.ReadRequest{FD: fh.(uintptr), Offset: offset, Num: n}, &reply)
	if err != nil {
		if err.Error() == "EOF" {
			data = make([]byte, 0)
			return data, 0
		}
		syserr = rpc_common.RPCErrorStringTOErrno(err)
		return nil, syserr
	}

	data = reply.Data
	return data, 0
}

func (d *FuseDockerClient) seek(ctx context.Context, fh fusefs.FileHandle, offset int64, whence int) (n int64, syserr syscall.Errno) {
	var reply rpc_common.SeekReply

	err := d.rpcClient.Call("DockerFuseFSOps.Seek", rpc_common.SeekRequest{FD: fh.(uintptr), Offset: offset, Whence: whence}, &reply)
	if err != nil {
		syserr = rpc_common.RPCErrorStringTOErrno(err)
		return
	}

	n = reply.Num
	return
}

func (d *FuseDockerClient) write(ctx context.Context, fh fusefs.FileHandle, offset int64, data []byte) (n int, syserr syscall.Errno) {
	var reply rpc_common.WriteReply

	err := d.rpcClient.Call("DockerFuseFSOps.Write", rpc_common.WriteRequest{FD: fh.(uintptr), Offset: offset, Data: data}, &reply)
	if err != nil {
		syserr = rpc_common.RPCErrorStringTOErrno(err)
		return
	}

	n = reply.Num
	return
}

func (d *FuseDockerClient) unlink(ctx context.Context, fullPath string) (syserr syscall.Errno) {
	var reply rpc_common.UnlinkReply

	err := d.rpcClient.Call("DockerFuseFSOps.Unlink", rpc_common.UnlinkRequest{FullPath: fullPath}, &reply)
	if err != nil {
		return rpc_common.RPCErrorStringTOErrno(err)
	}
	return
}

func (d *FuseDockerClient) fsync(ctx context.Context, fh fusefs.FileHandle, flags uint32) (syserr syscall.Errno) {
	var reply rpc_common.FsyncReply

	err := d.rpcClient.Call("DockerFuseFSOps.Fsync", rpc_common.FsyncRequest{FD: fh.(uintptr), Flags: flags}, &reply)
	if err != nil {
		return rpc_common.RPCErrorStringTOErrno(err)
	}
	return
}

func (d *FuseDockerClient) mkdir(ctx context.Context, fullPath string, mode fs.FileMode, attr *StatAttr) (syserr syscall.Errno) {
	var reply rpc_common.MkdirReply

	request := rpc_common.MkdirRequest{
		FullPath: fullPath,
		Mode:     mode,
	}
	err := d.rpcClient.Call("DockerFuseFSOps.Mkdir", request, &reply)
	if err != nil {
		return rpc_common.RPCErrorStringTOErrno(err)
	}
	attr.FuseAttr.Ino = reply.Ino
	attr.FuseAttr.Size = uint64(reply.Size)
	attr.FuseAttr.Blocks = uint64(reply.Blocks)
	attr.FuseAttr.Atime = uint64(reply.Atime)
	attr.FuseAttr.Mtime = uint64(reply.Mtime)
	attr.FuseAttr.Ctime = uint64(reply.Ctime)
	attr.FuseAttr.Mode = reply.Mode
	attr.FuseAttr.Nlink = reply.Nlink
	attr.FuseAttr.Owner.Uid = reply.Uid
	attr.FuseAttr.Owner.Gid = reply.Gid
	attr.LinkTarget = ""
	return
}

func (d *FuseDockerClient) rmdir(ctx context.Context, fullPath string) (syserr syscall.Errno) {
	var reply rpc_common.RmdirReply

	request := rpc_common.RmdirRequest{FullPath: fullPath}
	err := d.rpcClient.Call("DockerFuseFSOps.Rmdir", request, &reply)
	if err != nil {
		return rpc_common.RPCErrorStringTOErrno(err)
	}
	return
}

func (d *FuseDockerClient) rename(ctx context.Context, fullPath string, fullNewPath string, flags uint32) (syserr syscall.Errno) {
	var reply rpc_common.RenameReply

	request := rpc_common.RenameRequest{FullPath: fullPath, FullNewPath: fullNewPath, Flags: flags}
	err := d.rpcClient.Call("DockerFuseFSOps.Rename", request, &reply)
	if err != nil {
		return rpc_common.RPCErrorStringTOErrno(err)
	}
	return
}

func (d *FuseDockerClient) readlink(ctx context.Context, fullPath string) (linkTarget []byte, syserr syscall.Errno) {
	var reply rpc_common.ReadlinkReply

	request := rpc_common.ReadlinkRequest{FullPath: fullPath}
	err := d.rpcClient.Call("DockerFuseFSOps.Readlink", request, &reply)
	if err != nil {
		return []byte{}, rpc_common.RPCErrorStringTOErrno(err)
	}
	return []byte(reply.LinkTarget), 0
}

func (d *FuseDockerClient) link(ctx context.Context, oldFullPath string, newFullPath string) (syserr syscall.Errno) {
	var reply rpc_common.LinkReply

	request := rpc_common.LinkRequest{OldFullPath: oldFullPath, NewFullPath: newFullPath}
	err := d.rpcClient.Call("DockerFuseFSOps.Link", request, &reply)
	if err != nil {
		return rpc_common.RPCErrorStringTOErrno(err)
	}
	return 0
}

func (d *FuseDockerClient) symlink(ctx context.Context, oldFullPath string, newFullPath string) (syserr syscall.Errno) {
	var reply rpc_common.SymlinkReply

	request := rpc_common.SymlinkRequest{OldFullPath: oldFullPath, NewFullPath: newFullPath}
	err := d.rpcClient.Call("DockerFuseFSOps.Symlink", request, &reply)
	if err != nil {
		return rpc_common.RPCErrorStringTOErrno(err)
	}
	return 0
}

func (d *FuseDockerClient) setAttr(ctx context.Context, fullPath string, in *fuse.SetAttrIn, out *StatAttr) (syserr syscall.Errno) {
	var (
		request rpc_common.SetAttrRequest
		reply   rpc_common.SetAttrReply
	)

	request = rpc_common.SetAttrRequest{FullPath: fullPath}
	if atime, ok := in.GetATime(); ok {
		request.SetATime(atime)
	}
	if mtime, ok := in.GetMTime(); ok {
		request.SetMTime(mtime)
	}
	if uid, ok := in.GetUID(); ok {
		request.SetUid(uid)
	}
	if gid, ok := in.GetGID(); ok {
		request.SetGid(gid)
	}
	if mode, ok := in.GetMode(); ok {
		request.SetMode(mode)
	}
	if size, ok := in.GetSize(); ok {
		request.SetSize(size)
	}

	err := d.rpcClient.Call("DockerFuseFSOps.SetAttr", request, &reply)
	if err != nil {
		return rpc_common.RPCErrorStringTOErrno(err)
	}

	out.FuseAttr.Ino = reply.Ino
	out.FuseAttr.Size = uint64(reply.Size)
	out.FuseAttr.Blocks = uint64(reply.Blocks)
	out.FuseAttr.Atime = uint64(reply.Atime)
	out.FuseAttr.Mtime = uint64(reply.Mtime)
	out.FuseAttr.Ctime = uint64(reply.Ctime)
	out.FuseAttr.Mode = reply.Mode
	out.FuseAttr.Nlink = reply.Nlink
	out.FuseAttr.Owner.Uid = reply.Uid
	out.FuseAttr.Owner.Gid = reply.Gid
	out.LinkTarget = reply.LinkTarget
	return 0
}
