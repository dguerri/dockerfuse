package client

import (
	"context"
	"io/fs"
	"syscall"
	"testing"

	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockFuseDockerClient struct{ mock.Mock }

func (m *mockFuseDockerClient) disconnect() {
	m.Called()
}

func (m *mockFuseDockerClient) connectSatellite(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockFuseDockerClient) close(ctx context.Context, fh fusefs.FileHandle) syscall.Errno {
	args := m.Called(ctx, fh)
	if val, ok := args.Get(0).(syscall.Errno); ok {
		return val
	}
	return 0
}

func (m *mockFuseDockerClient) create(ctx context.Context, fullPath string, flags int, mode fs.FileMode, attr *statAttr) (fusefs.FileHandle, syscall.Errno) {
	args := m.Called(ctx, fullPath, flags, mode, attr)
	return args.Get(0).(fusefs.FileHandle), args.Get(1).(syscall.Errno)
}

func (m *mockFuseDockerClient) fsync(ctx context.Context, fh fusefs.FileHandle, flags uint32) syscall.Errno {
	args := m.Called(ctx, fh, flags)
	return args.Get(0).(syscall.Errno)
}

func (m *mockFuseDockerClient) link(ctx context.Context, oldFullPath, newFullPath string) syscall.Errno {
	args := m.Called(ctx, oldFullPath, newFullPath)
	return args.Get(0).(syscall.Errno)
}

func (m *mockFuseDockerClient) mkdir(ctx context.Context, fullPath string, mode fs.FileMode, attr *statAttr) syscall.Errno {
	args := m.Called(ctx, fullPath, mode, attr)
	return args.Get(0).(syscall.Errno)
}

func (m *mockFuseDockerClient) open(ctx context.Context, fullPath string, flags int, mode fs.FileMode) (fusefs.FileHandle, fs.FileMode, syscall.Errno) {
	args := m.Called(ctx, fullPath, flags, mode)
	return args.Get(0).(fusefs.FileHandle), args.Get(1).(fs.FileMode), args.Get(2).(syscall.Errno)
}

func (m *mockFuseDockerClient) read(ctx context.Context, fh fusefs.FileHandle, offset int64, n int) ([]byte, syscall.Errno) {
	args := m.Called(ctx, fh, offset, n)
	return args.Get(0).([]byte), args.Get(1).(syscall.Errno)
}

func (m *mockFuseDockerClient) readDir(ctx context.Context, fullPath string) (fusefs.DirStream, syscall.Errno) {
	args := m.Called(ctx, fullPath)
	return args.Get(0).(fusefs.DirStream), args.Get(1).(syscall.Errno)
}

func (m *mockFuseDockerClient) readlink(ctx context.Context, fullPath string) ([]byte, syscall.Errno) {
	args := m.Called(ctx, fullPath)
	return args.Get(0).([]byte), args.Get(1).(syscall.Errno)
}

func (m *mockFuseDockerClient) rename(ctx context.Context, fullPath, fullNewPath string, flags uint32) syscall.Errno {
	args := m.Called(ctx, fullPath, fullNewPath, flags)
	return args.Get(0).(syscall.Errno)
}

func (m *mockFuseDockerClient) rmdir(ctx context.Context, fullPath string) syscall.Errno {
	args := m.Called(ctx, fullPath)
	return args.Get(0).(syscall.Errno)
}

func (m *mockFuseDockerClient) seek(ctx context.Context, fh fusefs.FileHandle, offset int64, whence int) (int64, syscall.Errno) {
	args := m.Called(ctx, fh, offset, whence)
	return args.Get(0).(int64), args.Get(1).(syscall.Errno)
}

func (m *mockFuseDockerClient) setAttr(ctx context.Context, fullPath string, in *fuse.SetAttrIn, out *statAttr) syscall.Errno {
	args := m.Called(ctx, fullPath, in, out)
	return args.Get(0).(syscall.Errno)
}

func (m *mockFuseDockerClient) stat(ctx context.Context, fullPath string, attr *statAttr) syscall.Errno {
	args := m.Called(ctx, fullPath, attr)
	return args.Get(0).(syscall.Errno)
}

func (m *mockFuseDockerClient) symlink(ctx context.Context, oldFullPath, newFullPath string) syscall.Errno {
	args := m.Called(ctx, oldFullPath, newFullPath)
	return args.Get(0).(syscall.Errno)
}

func (m *mockFuseDockerClient) unlink(ctx context.Context, fullPath string) syscall.Errno {
	args := m.Called(ctx, fullPath)
	return args.Get(0).(syscall.Errno)
}

func (m *mockFuseDockerClient) write(ctx context.Context, fh fusefs.FileHandle, offset int64, data []byte) (int, syscall.Errno) {
	args := m.Called(ctx, fh, offset, data)
	return args.Int(0), args.Get(1).(syscall.Errno)
}

func TestNodeGetattrSuccess(t *testing.T) {
	var m mockFuseDockerClient
	n := NewNode(&m, "/path", "")
	m.On("stat", mock.Anything, "/path", mock.Anything).Run(func(args mock.Arguments) {
		attr := args.Get(2).(*statAttr)
		attr.FuseAttr = fuse.Attr{Ino: 10, Mode: fuse.S_IFREG, Size: 42}
	}).Return(syscall.Errno(0))

	var out fuse.AttrOut
	errno := n.Getattr(context.Background(), nil, &out)
	assert.Equal(t, syscall.Errno(0), errno)
	assert.Equal(t, uint64(10), out.Attr.Ino)
	assert.Equal(t, uint64(42), out.Attr.Size)
	m.AssertExpectations(t)
}

func TestNodeGetattrError(t *testing.T) {
	var m mockFuseDockerClient
	n := NewNode(&m, "/path", "")
	m.On("stat", mock.Anything, "/path", mock.Anything).Return(syscall.ENOENT)

	var out fuse.AttrOut
	errno := n.Getattr(context.Background(), nil, &out)
	assert.Equal(t, syscall.ENOENT, errno)
	m.AssertExpectations(t)
}

func TestNodeOpenAndRead(t *testing.T) {
	var m mockFuseDockerClient
	n := NewNode(&m, "/file", "")
	handle := fusefs.FileHandle(uintptr(1))
	m.On("open", mock.Anything, "/file", 0, fs.FileMode(0)).Return(handle, fs.FileMode(0644), syscall.Errno(0))
	fh, mode, err := n.Open(context.Background(), 0)
	assert.Equal(t, syscall.Errno(0), err)
	assert.Equal(t, handle, fh)
	assert.Equal(t, uint32(0644), mode)

	buf := make([]byte, 3)
	m.On("read", mock.Anything, handle, int64(0), len(buf)).Return([]byte("abc"), syscall.Errno(0))
	res, rerr := n.Read(context.Background(), handle, buf, 0)
	assert.Equal(t, syscall.Errno(0), rerr)
	data, _ := res.Bytes([]byte{})
	assert.Equal(t, []byte("abc"), data)

	m.AssertExpectations(t)
}

func TestNodeReadlinkReaddir(t *testing.T) {
	var m mockFuseDockerClient
	n := NewNode(&m, "/dir", "")
	m.On("readlink", mock.Anything, "/dir").Return([]byte("/target"), syscall.Errno(0))
	link, err := n.Readlink(context.Background())
	assert.Equal(t, []byte("/target"), link)
	assert.Equal(t, syscall.Errno(0), err)

	dirEntries := []fuse.DirEntry{{Name: "f1"}, {Name: "f2"}}
	m.On("readDir", mock.Anything, "/dir").Return(fusefs.NewListDirStream(dirEntries), syscall.Errno(0))
	ds, derr := n.Readdir(context.Background())
	assert.Equal(t, syscall.Errno(0), derr)
	names := []string{}
	for ds.HasNext() {
		e, _ := ds.Next()
		names = append(names, e.Name)
	}
	assert.Equal(t, []string{"f1", "f2"}, names)
	m.AssertExpectations(t)
}

func TestNodeWriteError(t *testing.T) {
	var m mockFuseDockerClient
	n := NewNode(&m, "/file", "")
	handle := fusefs.FileHandle(uintptr(1))
	m.On("write", mock.Anything, handle, int64(0), []byte("hi")).Return(0, syscall.EIO)
	nbytes, err := n.Write(context.Background(), handle, []byte("hi"), 0)
	assert.Equal(t, uint32(0), nbytes)
	assert.Equal(t, syscall.EIO, err)
	m.AssertExpectations(t)
}
