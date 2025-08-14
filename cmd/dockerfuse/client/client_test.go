package client

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/dguerri/dockerfuse/pkg/rpccommon"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/common"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockFS implements mock fileSystem for testing
type mockFS struct{ mock.Mock }

func (o *mockFS) Executable() (string, error) {
	args := o.Called()
	return args.String(0), args.Error(1)
}
func (o *mockFS) ReadFile(n string) ([]byte, error) {
	args := o.Called(n)
	return args.Get(0).([]byte), args.Error(1)
}

type mockRPCClientFactory struct{ mock.Mock }

func (m *mockRPCClientFactory) NewClient(conn io.ReadWriteCloser) rpcClient {
	args := m.Called(conn)
	return args.Get(0).(*mockRPCClient)
}

type mockRPCClient struct{ mock.Mock }

func (o *mockRPCClient) Call(sm string, a any, r any) error {
	args := o.Called(sm, a, r)
	return args.Error(0)
}

func (o *mockRPCClient) Close() error {
	args := o.Called()
	return args.Error(0)
}

type mockDockerClientFactory struct{ mock.Mock }

func (m *mockDockerClientFactory) NewClientWithOpts(ops ...client.Opt) (dockerClient, error) {
	args := m.Called(ops)
	return args.Get(0).(*mockDockerClient), args.Error(1)
}

type mockDockerClient struct{ mock.Mock }

func (dc *mockDockerClient) ContainerExecAttach(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error) {
	args := dc.Called(ctx, execID, config)
	return args.Get(0).(types.HijackedResponse), args.Error(1)
}
func (dc *mockDockerClient) ContainerExecCreate(ctx context.Context, container string, config container.ExecOptions) (common.IDResponse, error) {
	args := dc.Called(ctx, container, config)
	return args.Get(0).(common.IDResponse), args.Error(1)
}
func (dc *mockDockerClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	args := dc.Called(ctx, containerID)
	return args.Get(0).(container.InspectResponse), args.Error(1)
}
func (dc *mockDockerClient) CopyToContainer(ctx context.Context, containerID, dstPath string, content io.Reader, options container.CopyToContainerOptions) error {
	args := dc.Called(ctx, containerID, dstPath, content, options)
	return args.Error(0)
}
func (dc *mockDockerClient) ImageInspectWithRaw(ctx context.Context, imageID string) (image.InspectResponse, []byte, error) {
	args := dc.Called(ctx, imageID)
	return args.Get(0).(image.InspectResponse), args.Get(1).([]byte), args.Error(2)
}

func TestNewFuseDockerClient(t *testing.T) {
	// *** Setup
	var (
		mDC                                                               mockDockerClient
		mDCF                                                              mockDockerClientFactory
		mFS                                                               mockFS
		mRPCC                                                             mockRPCClient
		mRPCCF                                                            mockRPCClientFactory
		satelliteBinName, satelliteFullLocalPath, satelliteFullRemotePath string
		config                                                            container.ExecOptions
		err                                                               error
	)
	rpcCF = &mRPCCF  // Set mock RPC client factory
	dockerCF = &mDCF // Set mock RPC client factory
	dfFS = &mFS      // Set mock Filesystem

	satelliteBinName = fmt.Sprintf("%s_%s", satelliteBinPrefix, "arm64")
	satelliteFullLocalPath = filepath.Join("/test/pos/", satelliteBinName)
	satelliteFullRemotePath = filepath.Join(satelliteExecPath, satelliteBinName)

	// *** Test happy path
	mFS = mockFS{}
	mRPCC = mockRPCClient{}
	mRPCCF = mockRPCClientFactory{}
	mDC = mockDockerClient{}
	mDCF = mockDockerClientFactory{}

	mFS.On("ReadFile", satelliteFullLocalPath).Return([]byte("test executable content"), nil)
	mFS.On("Executable").Return("/test/pos/executable", nil)
	mRPCC.On("Close").Return(nil)
	mRPCCF.On("NewClient", nil).Return(&mRPCC)
	config = container.ExecOptions{
		AttachStderr: false,
		AttachStdout: true,
		AttachStdin:  true,
		Tty:          false,
		Cmd:          []string{satelliteFullRemotePath},
	}
	mDC.On("ContainerExecCreate", context.Background(), "test_container", config).Return(
		common.IDResponse{ID: "test_execid"}, nil)
	mDC.On("ContainerExecAttach", context.Background(), "test_execid", container.ExecStartOptions{Tty: true}).Return(
		types.HijackedResponse{Conn: nil}, nil)
	mDC.On("CopyToContainer", context.Background(), "test_container", satelliteExecPath,
		mock.AnythingOfType("*bufio.Reader"), container.CopyToContainerOptions{}).Return(nil)
	mDC.On("ImageInspectWithRaw", context.Background(), "test_container_image").Return(
		image.InspectResponse{Architecture: "arm64"}, []byte{}, nil)
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{Image: "test_container_image"},
	}, nil)
	mDCF.On("NewClientWithOpts", mock.Anything).Return(&mDC, nil)

	fdc, err := NewDockerFuseClient("test_container")

	assert.NoError(t, err)
	mRPCC.AssertNotCalled(t, "Close")
	mDCF.AssertExpectations(t)
	mDC.AssertExpectations(t)
	mFS.AssertExpectations(t)
	mRPCCF.AssertExpectations(t)

	// Test reconnection
	err = fdc.connectSatellite(context.Background())

	assert.NoError(t, err)
	mRPCC.AssertExpectations(t)

	// *** Test error on NewClientWithOpts
	mFS = mockFS{}
	mRPCC = mockRPCClient{}
	mRPCCF = mockRPCClientFactory{}
	mDC = mockDockerClient{}
	mDCF = mockDockerClientFactory{}

	newClientWithOptsError := fmt.Errorf("error on NewClientWithOpts")
	mDCF.On("NewClientWithOpts", mock.Anything).Return(&mDC, newClientWithOptsError)

	_, err = NewDockerFuseClient("test_container")

	if assert.Error(t, err) {
		assert.Equal(t, newClientWithOptsError, err)
	}
	mDCF.AssertExpectations(t)

	// *** Test error on ContainerInspect
	mFS = mockFS{}
	mRPCC = mockRPCClient{}
	mRPCCF = mockRPCClientFactory{}
	mDC = mockDockerClient{}
	mDCF = mockDockerClientFactory{}

	containerInspectError := fmt.Errorf("error on ContainerInspect")
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(container.InspectResponse{}, containerInspectError)
	mDCF.On("NewClientWithOpts", mock.Anything).Return(&mDC, nil)

	_, err = NewDockerFuseClient("test_container")

	if assert.Error(t, err) {
		assert.Equal(t,
			fmt.Errorf("error copying docker-fuse satellite to remote container: %s", containerInspectError.Error()),
			err,
		)
	}
	mDCF.AssertExpectations(t)

	// *** Test error on ContainerInspect
	mFS = mockFS{}
	mRPCC = mockRPCClient{}
	mRPCCF = mockRPCClientFactory{}
	mDC = mockDockerClient{}
	mDCF = mockDockerClientFactory{}

	imageInspectWithRawError := fmt.Errorf("error on ImageInspectWithRaw")
	mDC.On("ImageInspectWithRaw", context.Background(), "test_container_image").Return(
		image.InspectResponse{}, []byte{}, imageInspectWithRawError)
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{Image: "test_container_image"},
	}, nil)
	mDCF.On("NewClientWithOpts", mock.Anything).Return(&mDC, nil)

	_, err = NewDockerFuseClient("test_container")

	if assert.Error(t, err) {
		assert.Equal(t,
			fmt.Errorf("error copying docker-fuse satellite to remote container: %s", imageInspectWithRawError.Error()),
			err,
		)
	}
	mDCF.AssertExpectations(t)

	// *** Test error on invalid architecture
	mFS = mockFS{}
	mRPCC = mockRPCClient{}
	mRPCCF = mockRPCClientFactory{}
	mDC = mockDockerClient{}
	mDCF = mockDockerClientFactory{}

	mDC.On("ImageInspectWithRaw", context.Background(), "test_container_image").Return(
		image.InspectResponse{Architecture: "invalidarc64"}, []byte{}, nil)
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{Image: "test_container_image"},
	}, nil)
	mDCF.On("NewClientWithOpts", mock.Anything).Return(&mDC, nil)

	_, err = NewDockerFuseClient("test_container")

	if assert.Error(t, err) {
		assert.Equal(t,
			fmt.Errorf("error copying docker-fuse satellite to remote container: unsupported architecture: invalidarc64 (use arm64 or amd64)"),
			err,
		)
	}
	mDCF.AssertExpectations(t)

	// *** Test error on os.Executable()
	mFS = mockFS{}
	mRPCC = mockRPCClient{}
	mRPCCF = mockRPCClientFactory{}
	mDC = mockDockerClient{}
	mDCF = mockDockerClientFactory{}

	osExecutableError := fmt.Errorf("error on os.Executable()")
	mFS.On("Executable").Return("/test/pos/executable", osExecutableError)
	mDC.On("ImageInspectWithRaw", context.Background(), "test_container_image").Return(
		image.InspectResponse{Architecture: "arm64"}, []byte{}, nil)
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{Image: "test_container_image"},
	}, nil)
	mDCF.On("NewClientWithOpts", mock.Anything).Return(&mDC, nil)

	_, err = NewDockerFuseClient("test_container")

	if assert.Error(t, err) {
		assert.Equal(t,
			fmt.Errorf("error copying docker-fuse satellite to remote container: %s", osExecutableError.Error()),
			err,
		)
	}
	mDCF.AssertExpectations(t)

	// *** Test error on os.ReadFile()
	mFS = mockFS{}
	mRPCC = mockRPCClient{}
	mRPCCF = mockRPCClientFactory{}
	mDC = mockDockerClient{}
	mDCF = mockDockerClientFactory{}

	osReadFileError := fmt.Errorf("error on os.ReadFile()")
	mFS.On("ReadFile", satelliteFullLocalPath).Return([]byte{}, osReadFileError)
	mFS.On("Executable").Return("/test/pos/executable", nil)
	mDC.On("ImageInspectWithRaw", context.Background(), "test_container_image").Return(
		image.InspectResponse{Architecture: "arm64"}, []byte{}, nil)
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{Image: "test_container_image"},
	}, nil)
	mDCF.On("NewClientWithOpts", mock.Anything).Return(&mDC, nil)

	_, err = NewDockerFuseClient("test_container")

	if assert.Error(t, err) {
		assert.Equal(t,
			fmt.Errorf("error copying docker-fuse satellite to remote container: %s", osReadFileError.Error()),
			err,
		)
	}
	mDCF.AssertExpectations(t)

	// *** Test error on CopyToContainer
	mFS = mockFS{}
	mRPCC = mockRPCClient{}
	mRPCCF = mockRPCClientFactory{}
	mDC = mockDockerClient{}
	mDCF = mockDockerClientFactory{}

	copyToContainerError := fmt.Errorf("error on CopyToContainer")
	mDC.On("CopyToContainer", context.Background(), "test_container", satelliteExecPath,
		mock.AnythingOfType("*bufio.Reader"), container.CopyToContainerOptions{}).Return(copyToContainerError)
	mFS.On("ReadFile", satelliteFullLocalPath).Return([]byte("test executable content"), nil)
	mFS.On("Executable").Return("/test/pos/executable", nil)
	mDC.On("ImageInspectWithRaw", context.Background(), "test_container_image").Return(
		image.InspectResponse{Architecture: "arm64"}, []byte{}, nil)
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{Image: "test_container_image"},
	}, nil)
	mDCF.On("NewClientWithOpts", mock.Anything).Return(&mDC, nil)

	_, err = NewDockerFuseClient("test_container")

	if assert.Error(t, err) {
		assert.Equal(t,
			fmt.Errorf("error copying docker-fuse satellite to remote container: %s", copyToContainerError.Error()),
			err,
		)
	}
	mDCF.AssertExpectations(t)

	// *** Test error on ContainerExecCreate
	mFS = mockFS{}
	mRPCC = mockRPCClient{}
	mRPCCF = mockRPCClientFactory{}
	mDC = mockDockerClient{}
	mDCF = mockDockerClientFactory{}

	containerExecCreateError := fmt.Errorf("error on ContainerExecCreate")
	mDC.On("ContainerExecCreate", context.Background(), "test_container", config).Return(
		common.IDResponse{}, containerExecCreateError)
	mDC.On("CopyToContainer", context.Background(), "test_container", satelliteExecPath,
		mock.AnythingOfType("*bufio.Reader"), container.CopyToContainerOptions{}).Return(nil)
	mFS.On("ReadFile", satelliteFullLocalPath).Return([]byte("test executable content"), nil)
	mFS.On("Executable").Return("/test/pos/executable", nil)
	mDC.On("ImageInspectWithRaw", context.Background(), "test_container_image").Return(
		image.InspectResponse{Architecture: "arm64"}, []byte{}, nil)
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{Image: "test_container_image"},
	}, nil)
	mDCF.On("NewClientWithOpts", mock.Anything).Return(&mDC, nil)

	_, err = NewDockerFuseClient("test_container")

	if assert.Error(t, err) {
		assert.Equal(t,
			fmt.Errorf("error connecting to docker-fuse satellite: %s", containerExecCreateError.Error()),
			err,
		)
	}
	mDCF.AssertExpectations(t)

	// *** Test error on ContainerExecAttach
	mFS = mockFS{}
	mRPCC = mockRPCClient{}
	mRPCCF = mockRPCClientFactory{}
	mDC = mockDockerClient{}
	mDCF = mockDockerClientFactory{}

	containerExecAttachError := fmt.Errorf("error on ContainerExecAttach")
	mDC.On("ContainerExecAttach", context.Background(), "test_execid", container.ExecStartOptions{Tty: true}).Return(
		types.HijackedResponse{}, containerExecAttachError)
	mDC.On("ContainerExecCreate", context.Background(), "test_container", config).Return(
		common.IDResponse{ID: "test_execid"}, nil)
	mDC.On("CopyToContainer", context.Background(), "test_container", satelliteExecPath,
		mock.AnythingOfType("*bufio.Reader"), container.CopyToContainerOptions{}).Return(nil)
	mFS.On("ReadFile", satelliteFullLocalPath).Return([]byte("test executable content"), nil)
	mFS.On("Executable").Return("/test/pos/executable", nil)
	mDC.On("ImageInspectWithRaw", context.Background(), "test_container_image").Return(
		image.InspectResponse{Architecture: "arm64"}, []byte{}, nil)
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{Image: "test_container_image"},
	}, nil)
	mDCF.On("NewClientWithOpts", mock.Anything).Return(&mDC, nil)

	_, err = NewDockerFuseClient("test_container")

	if assert.Error(t, err) {
		assert.Equal(t,
			fmt.Errorf("error connecting to docker-fuse satellite: %s", containerExecAttachError.Error()),
			err,
		)
	}
	mDCF.AssertExpectations(t)

}

func TestDockerFuseClientStat(t *testing.T) {
	var mRPCC mockRPCClient
	fdc := &DockerFuseClient{rpcClient: &mRPCC}

	expected := rpccommon.StatReply{
		Mode:       0755,
		Nlink:      1,
		Ino:        42,
		UID:        1000,
		GID:        1000,
		Atime:      1,
		Mtime:      2,
		Ctime:      3,
		Size:       64,
		Blocks:     1,
		Blksize:    4096,
		LinkTarget: "link",
	}

	mRPCC.On("Call", "DockerFuseFSOps.Stat", rpccommon.StatRequest{FullPath: "/test"}, mock.Anything).
		Run(func(args mock.Arguments) {
			reply := args.Get(2).(*rpccommon.StatReply)
			*reply = expected
		}).Return(nil)

	var attr statAttr
	errno := fdc.stat(context.Background(), "/test", &attr)

	assert.Equal(t, syscall.Errno(0), errno)
	assert.Equal(t, expected.Ino, attr.FuseAttr.Ino)
	assert.Equal(t, uint64(expected.Size), attr.FuseAttr.Size)
	assert.Equal(t, uint64(expected.Blocks), attr.FuseAttr.Blocks)
	assert.Equal(t, expected.Mode, attr.FuseAttr.Mode)
	assert.Equal(t, expected.Nlink, attr.FuseAttr.Nlink)
	assert.Equal(t, expected.UID, attr.FuseAttr.Owner.Uid)
	assert.Equal(t, expected.GID, attr.FuseAttr.Owner.Gid)
	assert.Equal(t, expected.LinkTarget, attr.LinkTarget)

	mRPCC.AssertExpectations(t)
}

func TestDockerFuseClientStatError(t *testing.T) {
	var mRPCC mockRPCClient
	fdc := &DockerFuseClient{rpcClient: &mRPCC}

	mRPCC.On("Call", "DockerFuseFSOps.Stat", rpccommon.StatRequest{FullPath: "/enoent"}, mock.Anything).
		Return(fmt.Errorf("errno: ENOENT"))

	var attr statAttr
	errno := fdc.stat(context.Background(), "/enoent", &attr)

	assert.Equal(t, syscall.ENOENT, errno)
	mRPCC.AssertExpectations(t)
}

func TestDockerFuseClientReadDir(t *testing.T) {
	var mRPCC mockRPCClient
	fdc := &DockerFuseClient{rpcClient: &mRPCC}

	reply := rpccommon.ReadDirReply{DirEntries: []rpccommon.DirEntry{
		{Ino: 1, Name: "."},
		{Ino: 2, Name: ".."},
		{Ino: 3, Name: "file", Mode: 0644},
		{Ino: 4, Name: "dir", Mode: 0755},
	}}

	mRPCC.On("Call", "DockerFuseFSOps.ReadDir", rpccommon.StatRequest{FullPath: "/dir"}, mock.Anything).
		Run(func(args mock.Arguments) {
			r := args.Get(2).(*rpccommon.ReadDirReply)
			*r = reply
		}).Return(nil)

	ds, errno := fdc.readDir(context.Background(), "/dir")

	assert.Equal(t, syscall.Errno(0), errno)

	var got []string
	for ds.HasNext() {
		e, _ := ds.Next()
		got = append(got, e.Name)
	}

	assert.Equal(t, []string{"file", "dir"}, got)
	mRPCC.AssertExpectations(t)
}

func TestDockerFuseClientReadDirError(t *testing.T) {
	var mRPCC mockRPCClient
	fdc := &DockerFuseClient{rpcClient: &mRPCC}

	mRPCC.On("Call", "DockerFuseFSOps.ReadDir", rpccommon.StatRequest{FullPath: "/err"}, mock.Anything).
		Return(fmt.Errorf("errno: EACCES"))

	_, errno := fdc.readDir(context.Background(), "/err")

	assert.Equal(t, syscall.EACCES, errno)
	mRPCC.AssertExpectations(t)
}

func TestDockerFuseClientCreate(t *testing.T) {
	var mRPCC mockRPCClient
	fdc := &DockerFuseClient{rpcClient: &mRPCC}
	mRPCC.On("Call", "DockerFuseFSOps.Open", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		reply := args.Get(2).(*rpccommon.OpenReply)
		*reply = rpccommon.OpenReply{FD: 1, StatReply: rpccommon.StatReply{Mode: 0644}}
	}).Return(nil)
	var attr statAttr
	fh, err := fdc.create(context.Background(), "/f", 0, 0644, &attr)
	assert.Equal(t, fusefs.FileHandle(uintptr(1)), fh)
	assert.Equal(t, syscall.Errno(0), err)
	assert.Equal(t, uint32(0644), attr.FuseAttr.Mode)
	mRPCC.AssertExpectations(t)
}

func TestDockerFuseClientOpenClose(t *testing.T) {
	var mRPCC mockRPCClient
	fdc := &DockerFuseClient{rpcClient: &mRPCC}
	mRPCC.On("Call", "DockerFuseFSOps.Open", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		reply := args.Get(2).(*rpccommon.OpenReply)
		*reply = rpccommon.OpenReply{FD: 2, StatReply: rpccommon.StatReply{Mode: 0600}}
	}).Return(nil)
	fh, mode, err := fdc.open(context.Background(), "/f", 0, 0)
	assert.Equal(t, fusefs.FileHandle(uintptr(2)), fh)
	assert.Equal(t, fs.FileMode(0600), mode)
	assert.Equal(t, syscall.Errno(0), err)
	mRPCC.On("Call", "DockerFuseFSOps.Close", rpccommon.CloseRequest{FD: fh.(uintptr)}, mock.Anything).Return(nil)
	cerr := fdc.close(context.Background(), fh)
	assert.Equal(t, syscall.Errno(0), cerr)
	mRPCC.AssertExpectations(t)
}

func TestDockerFuseClientReadSeekWrite(t *testing.T) {
	var mRPCC mockRPCClient
	fdc := &DockerFuseClient{rpcClient: &mRPCC}
	mRPCC.On("Call", "DockerFuseFSOps.Read", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		r := args.Get(2).(*rpccommon.ReadReply)
		*r = rpccommon.ReadReply{Data: []byte("a")}
	}).Return(nil)
	data, err := fdc.read(context.Background(), fusefs.FileHandle(uintptr(1)), 0, 1)
	assert.Equal(t, []byte("a"), data)
	assert.Equal(t, syscall.Errno(0), err)
	mRPCC.On("Call", "DockerFuseFSOps.Seek", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		r := args.Get(2).(*rpccommon.SeekReply)
		*r = rpccommon.SeekReply{Num: 3}
	}).Return(nil)
	n, serr := fdc.seek(context.Background(), fusefs.FileHandle(uintptr(1)), 3, 0)
	assert.Equal(t, int64(3), n)
	assert.Equal(t, syscall.Errno(0), serr)
	mRPCC.On("Call", "DockerFuseFSOps.Write", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		r := args.Get(2).(*rpccommon.WriteReply)
		*r = rpccommon.WriteReply{Num: 1}
	}).Return(nil)
	wn, werr := fdc.write(context.Background(), fusefs.FileHandle(uintptr(1)), 0, []byte("a"))
	assert.Equal(t, 1, wn)
	assert.Equal(t, syscall.Errno(0), werr)
	mRPCC.AssertExpectations(t)
}

func TestDockerFuseClientOtherOps(t *testing.T) {
	var m mockRPCClient
	fdc := &DockerFuseClient{rpcClient: &m}
	m.On("Call", "DockerFuseFSOps.Unlink", rpccommon.UnlinkRequest{FullPath: "/a"}, mock.Anything).Return(nil)
	assert.Equal(t, syscall.Errno(0), fdc.unlink(context.Background(), "/a"))
	m.On("Call", "DockerFuseFSOps.Fsync", mock.Anything, mock.Anything).Return(nil)
	assert.Equal(t, syscall.Errno(0), fdc.fsync(context.Background(), fusefs.FileHandle(uintptr(1)), 0))
	m.On("Call", "DockerFuseFSOps.Mkdir", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		r := args.Get(2).(*rpccommon.MkdirReply)
		*r = rpccommon.MkdirReply{Ino: 1}
	}).Return(nil)
	var attr statAttr
	assert.Equal(t, syscall.Errno(0), fdc.mkdir(context.Background(), "/d", 0755, &attr))
	m.On("Call", "DockerFuseFSOps.Rmdir", rpccommon.RmdirRequest{FullPath: "/d"}, mock.Anything).Return(nil)
	assert.Equal(t, syscall.Errno(0), fdc.rmdir(context.Background(), "/d"))
	m.On("Call", "DockerFuseFSOps.Rename", rpccommon.RenameRequest{FullPath: "/a", FullNewPath: "/b", Flags: 0}, mock.Anything).Return(nil)
	assert.Equal(t, syscall.Errno(0), fdc.rename(context.Background(), "/a", "/b", 0))
	m.On("Call", "DockerFuseFSOps.Readlink", rpccommon.ReadlinkRequest{FullPath: "/l"}, mock.Anything).Run(func(args mock.Arguments) {
		r := args.Get(2).(*rpccommon.ReadlinkReply)
		*r = rpccommon.ReadlinkReply{LinkTarget: "t"}
	}).Return(nil)
	link, err := fdc.readlink(context.Background(), "/l")
	assert.Equal(t, []byte("t"), link)
	assert.Equal(t, syscall.Errno(0), err)
	m.On("Call", "DockerFuseFSOps.Link", rpccommon.LinkRequest{OldFullPath: "/o", NewFullPath: "/n"}, mock.Anything).Return(nil)
	assert.Equal(t, syscall.Errno(0), fdc.link(context.Background(), "/o", "/n"))
	m.On("Call", "DockerFuseFSOps.Symlink", rpccommon.SymlinkRequest{OldFullPath: "/o", NewFullPath: "/s"}, mock.Anything).Return(nil)
	assert.Equal(t, syscall.Errno(0), fdc.symlink(context.Background(), "/o", "/s"))
	m.On("Call", "DockerFuseFSOps.SetAttr", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		r := args.Get(2).(*rpccommon.SetAttrReply)
		*r = rpccommon.SetAttrReply{Ino: 5}
	}).Return(nil)
	var out statAttr
	in := &fuse.SetAttrIn{SetAttrInCommon: fuse.SetAttrInCommon{Valid: fuse.FATTR_SIZE, Size: 1}}
	assert.Equal(t, syscall.Errno(0), fdc.setAttr(context.Background(), "/file", in, &out))
	m.AssertExpectations(t)
}
