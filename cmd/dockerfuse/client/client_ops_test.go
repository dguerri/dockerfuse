package client

import (
	"context"
	"fmt"
	"syscall"
	"testing"

	"github.com/dguerri/dockerfuse/pkg/rpccommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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
