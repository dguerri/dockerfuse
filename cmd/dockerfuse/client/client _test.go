package client

import (
	"context"
	"dockerfuse/pkg/rpc_common"
	"fmt"
	"io"
	"syscall"
	"testing"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockRPCClientFactory struct {
	mRPCC *mockRPCClient
}

func (m *mockRPCClientFactory) NewClient(conn io.ReadWriteCloser) rpcClient {
	m.mRPCC = new(mockRPCClient)
	return m.mRPCC
}

type mockRPCClient struct {
	mock.Mock
}

func (o *mockRPCClient) Call(sm string, a any, r any) error {
	args := o.Called(sm, a, r)
	return args.Error(0)
}

func (o *mockRPCClient) Close() error {
	args := o.Called()
	return args.Error(0)
}

// TODO
/*
 type mockDockerClientFactory struct {
	mDC *mockDockerClient
}

func (m *mockDockerClientFactory) NewClientWithOpts(ops ...client.Opt) (mockDockerClient, error) {
	m.mDC = new(mockDockerClient)
	return *m.mDC, nil
}

type mockDockerClient struct {
	mock.Mock
}

func (dc *mockDockerClient) ContainerExecAttach(ctx context.Context, execID string, config types.ExecStartCheck) (types.HijackedResponse, error) {}
func (dc *mockDockerClient) ContainerExecCreate(ctx context.Context, container string, config types.ExecConfig) (types.IDResponse, error) {}
func (dc *mockDockerClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {}
func (dc *mockDockerClient) CopyToContainer(ctx context.Context, containerID, dstPath string, content io.Reader, options types.CopyToContainerOptions) error {}
func (dc *mockDockerClient) ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error) {}
*/

func TestStat(t *testing.T) {
	//Setup
	mRPCCF := new(mockRPCClientFactory)
	rpcCF = mRPCCF

	// TODO: replace with NewFuseDockerClient() once mockDockerClient is implemented
	mRPCCF.NewClient(nil)
	rpcC := mRPCCF.mRPCC
	fDC := FuseDockerClient{rpcClient: rpcC}

	var (
		mockCallCall *mock.Call
		reply        rpc_common.StatReply
		syserr       syscall.Errno
		statAttr     StatAttr
	)

	// Testing stat error
	mockCallCall = rpcC.On("Call", "DockerFuseFSOps.Stat", rpc_common.StatRequest{FullPath: "/test/error"}, &reply).Return(fmt.Errorf("errno: ENOENT"))

	syserr = fDC.stat(context.TODO(), "/test/error", &statAttr)

	if assert.Error(t, syserr) {
		assert.Equal(t, syscall.ENOENT, syserr)
	}
	rpcC.AssertExpectations(t)
	rpcC.AssertCalled(t, "Call", "DockerFuseFSOps.Stat", rpc_common.StatRequest{FullPath: "/test/error"}, &reply)
	assert.Equal(t, rpc_common.StatReply{}, reply)

	mockCallCall.Unset()
	reply = rpc_common.StatReply{}
	statAttr = StatAttr{}

	// Testing stat() on symlink
	mockCallCall = rpcC.On("Call", "DockerFuseFSOps.Stat", rpc_common.StatRequest{FullPath: "/test/symlink"}, &reply).Return(nil)
	mockCallCall.Run(func(args mock.Arguments) {
		arg := args.Get(2).(*rpc_common.StatReply)
		*arg = rpc_common.StatReply{
			Mode:       0666,
			Nlink:      1,
			Ino:        29,
			Uid:        1,
			Gid:        2,
			Atime:      2929,
			Mtime:      2930,
			Ctime:      2931,
			Size:       29,
			Blocks:     29,
			Blksize:    1024,
			LinkTarget: "/test/linktarget",
		}
	})

	syserr = fDC.stat(context.TODO(), "/test/symlink", &statAttr)

	assert.Equal(t, syserr, syscall.Errno(0))
	rpcC.AssertExpectations(t)
	rpcC.AssertCalled(t, "Call", "DockerFuseFSOps.Stat", rpc_common.StatRequest{FullPath: "/test/symlink"}, mock.Anything)
	assert.Equal(t, StatAttr{
		FuseAttr: fuse.Attr{
			Mode:   0666,
			Nlink:  1,
			Ino:    29,
			Owner:  fuse.Owner{Uid: 1, Gid: 2},
			Atime:  2929,
			Mtime:  2930,
			Ctime:  2931,
			Size:   29,
			Blocks: 29,
		},
		LinkTarget: "/test/linktarget",
	}, statAttr)

	mockCallCall.Unset()
	reply = rpc_common.StatReply{}
	statAttr = StatAttr{}

	// Testing stat() on regular file
	mockCallCall = rpcC.On("Call", "DockerFuseFSOps.Stat", rpc_common.StatRequest{FullPath: "/test/reg"}, &reply).Return(nil)
	mockCallCall.Run(func(args mock.Arguments) {
		arg := args.Get(2).(*rpc_common.StatReply)
		*arg = rpc_common.StatReply{
			Mode:       0760,
			Nlink:      1,
			Ino:        29,
			Uid:        1,
			Gid:        2,
			Atime:      2929,
			Mtime:      2930,
			Ctime:      2931,
			Size:       29,
			Blocks:     29,
			Blksize:    1024,
			LinkTarget: "",
		}
	})

	syserr = fDC.stat(context.TODO(), "/test/reg", &statAttr)

	assert.Equal(t, syserr, syscall.Errno(0))
	rpcC.AssertExpectations(t)
	rpcC.AssertCalled(t, "Call", "DockerFuseFSOps.Stat", rpc_common.StatRequest{FullPath: "/test/reg"}, mock.Anything)
	assert.Equal(t, StatAttr{
		FuseAttr: fuse.Attr{
			Mode:   0760,
			Nlink:  1,
			Ino:    29,
			Owner:  fuse.Owner{Uid: 1, Gid: 2},
			Atime:  2929,
			Mtime:  2930,
			Ctime:  2931,
			Size:   29,
			Blocks: 29,
		},
		LinkTarget: "",
	}, statAttr)

	mockCallCall.Unset()
	reply = rpc_common.StatReply{}
	statAttr = StatAttr{}
}
