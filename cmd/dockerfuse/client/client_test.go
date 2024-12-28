package client

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
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
func (dc *mockDockerClient) ContainerExecCreate(ctx context.Context, container string, config container.ExecOptions) (types.IDResponse, error) {
	args := dc.Called(ctx, container, config)
	return args.Get(0).(types.IDResponse), args.Error(1)
}
func (dc *mockDockerClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	args := dc.Called(ctx, containerID)
	return args.Get(0).(types.ContainerJSON), args.Error(1)
}
func (dc *mockDockerClient) CopyToContainer(ctx context.Context, containerID, dstPath string, content io.Reader, options container.CopyToContainerOptions) error {
	args := dc.Called(ctx, containerID, dstPath, content, options)
	return args.Error(0)
}
func (dc *mockDockerClient) ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error) {
	args := dc.Called(ctx, imageID)
	return args.Get(0).(types.ImageInspect), args.Get(1).([]byte), args.Error(2)
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
		types.IDResponse{ID: "test_execid"}, nil)
	mDC.On("ContainerExecAttach", context.Background(), "test_execid", container.ExecStartOptions{Tty: true}).Return(
		types.HijackedResponse{Conn: nil}, nil)
	mDC.On("CopyToContainer", context.Background(), "test_container", satelliteExecPath,
		mock.AnythingOfType("*bufio.Reader"), container.CopyToContainerOptions{}).Return(nil)
	mDC.On("ImageInspectWithRaw", context.Background(), "test_container_image").Return(
		types.ImageInspect{Architecture: "arm64"}, []byte{}, nil)
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{Image: "test_container_image"},
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
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(types.ContainerJSON{}, containerInspectError)
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
		types.ImageInspect{}, []byte{}, imageInspectWithRawError)
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{Image: "test_container_image"},
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
		types.ImageInspect{Architecture: "invalidarc64"}, []byte{}, nil)
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{Image: "test_container_image"},
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
		types.ImageInspect{Architecture: "arm64"}, []byte{}, nil)
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{Image: "test_container_image"},
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
		types.ImageInspect{Architecture: "arm64"}, []byte{}, nil)
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{Image: "test_container_image"},
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
		types.ImageInspect{Architecture: "arm64"}, []byte{}, nil)
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{Image: "test_container_image"},
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
		types.IDResponse{}, containerExecCreateError)
	mDC.On("CopyToContainer", context.Background(), "test_container", satelliteExecPath,
		mock.AnythingOfType("*bufio.Reader"), container.CopyToContainerOptions{}).Return(nil)
	mFS.On("ReadFile", satelliteFullLocalPath).Return([]byte("test executable content"), nil)
	mFS.On("Executable").Return("/test/pos/executable", nil)
	mDC.On("ImageInspectWithRaw", context.Background(), "test_container_image").Return(
		types.ImageInspect{Architecture: "arm64"}, []byte{}, nil)
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{Image: "test_container_image"},
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
		types.IDResponse{ID: "test_execid"}, nil)
	mDC.On("CopyToContainer", context.Background(), "test_container", satelliteExecPath,
		mock.AnythingOfType("*bufio.Reader"), container.CopyToContainerOptions{}).Return(nil)
	mFS.On("ReadFile", satelliteFullLocalPath).Return([]byte("test executable content"), nil)
	mFS.On("Executable").Return("/test/pos/executable", nil)
	mDC.On("ImageInspectWithRaw", context.Background(), "test_container_image").Return(
		types.ImageInspect{Architecture: "arm64"}, []byte{}, nil)
	mDC.On("ContainerInspect", context.Background(), "test_container").Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{Image: "test_container_image"},
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
