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
func TestNewNode(t *testing.T) {
	var m mockFuseDockerClient
	n := NewNode(&m, "/a", "b")
	assert.Equal(t, "/a", n.fullPath)
	assert.Equal(t, []byte("b"), n.Data)
	assert.Equal(t, &m, n.fuseDockerClient)
}

func TestNodeCreateMkdirAndMore(t *testing.T) {
	var m mockFuseDockerClient
	parent := NewNode(&m, "/dir", "")
	fusefs.NewNodeFS(parent, &fusefs.Options{})
	m.On("create", mock.Anything, "/dir/f", 0, fs.FileMode(0644), mock.Anything).Run(func(args mock.Arguments) {
		attr := args.Get(4).(*statAttr)
		attr.FuseAttr = fuse.Attr{Ino: 1}
	}).Return(fusefs.FileHandle(uintptr(1)), syscall.Errno(0))
	var out fuse.EntryOut
	newInode, fh, _, errno := parent.Create(context.Background(), "f", 0, 0644, &out)
	assert.NotNil(t, newInode)
	assert.Equal(t, fusefs.FileHandle(uintptr(1)), fh)
	assert.Equal(t, syscall.Errno(0), errno)

	m.On("mkdir", mock.Anything, "/dir/d", fs.FileMode(0755), mock.Anything).Run(func(args mock.Arguments) {
		attr := args.Get(3).(*statAttr)
		attr.FuseAttr = fuse.Attr{Ino: 2}
	}).Return(syscall.Errno(0))
	newInode2, err := parent.Mkdir(context.Background(), "d", 0755, &out)
	assert.NotNil(t, newInode2)
	assert.Equal(t, syscall.Errno(0), err)

	m.On("seek", mock.Anything, fh, int64(0), 0).Return(int64(0), syscall.Errno(0))
	n, serr := parent.Lseek(context.Background(), fh, 0, 0)
	assert.Equal(t, uint64(0), n)
	assert.Equal(t, syscall.Errno(0), serr)
}

func TestNodeLinkSymlinkRenameEtc(t *testing.T) {
	var m mockFuseDockerClient
	dir := NewNode(&m, "/dir", "")
	fusefs.NewNodeFS(dir, &fusefs.Options{})
	target := NewNode(&m, "/dir/t", "")
	m.On("link", mock.Anything, "/dir/t", "/dir/l").Return(syscall.Errno(0))
	m.On("stat", mock.Anything, "/dir/l", mock.Anything).Run(func(args mock.Arguments) {
		attr := args.Get(2).(*statAttr)
		attr.FuseAttr = fuse.Attr{Ino: 3}
	}).Return(syscall.Errno(0))
	var out fuse.EntryOut
	n, errno := dir.Link(context.Background(), target, "l", &out)
	assert.NotNil(t, n)
	assert.Equal(t, syscall.Errno(0), errno)

	m.On("symlink", mock.Anything, "/dir/t", "/dir/s").Return(syscall.Errno(0))
	n2, syErr := dir.Symlink(context.Background(), "/dir/t", "s", &out)
	assert.NotNil(t, n2)
	assert.Equal(t, syscall.Errno(0), syErr)

	m.On("rename", mock.Anything, "/dir/t", "r", uint32(0)).Return(syscall.Errno(0))
	rerr := dir.Rename(context.Background(), "t", dir, "r", 0)
	assert.Equal(t, syscall.Errno(0), rerr)

	m.On("rmdir", mock.Anything, "/dir/x").Return(syscall.Errno(0))
	rderr := dir.Rmdir(context.Background(), "x")
	assert.Equal(t, syscall.Errno(0), rderr)

	m.On("unlink", mock.Anything, "/dir/u").Return(syscall.Errno(0))
	uerr := dir.Unlink(context.Background(), "u")
	assert.Equal(t, syscall.Errno(0), uerr)
}

func TestNodeSetattrWriteSuccess(t *testing.T) {
	var m mockFuseDockerClient
	n := NewNode(&m, "/f", "")
	m.On("setAttr", mock.Anything, "/f", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		out := args.Get(3).(*statAttr)
		out.FuseAttr = fuse.Attr{Ino: 1}
	}).Return(syscall.Errno(0))
	var out fuse.AttrOut
	errno := n.Setattr(context.Background(), nil, &fuse.SetAttrIn{SetAttrInCommon: fuse.SetAttrInCommon{Valid: fuse.FATTR_SIZE, Size: 1}}, &out)
	assert.Equal(t, syscall.Errno(0), errno)

	handle := fusefs.FileHandle(uintptr(1))
	m.On("write", mock.Anything, handle, int64(0), []byte("hi")).Return(2, syscall.Errno(0))
	wn, werr := n.Write(context.Background(), handle, []byte("hi"), 0)
	assert.Equal(t, uint32(2), wn)
	assert.Equal(t, syscall.Errno(0), werr)
}

func TestNodeLookupSuccessTypes(t *testing.T) {
	tests := []struct {
		name string
		mode uint32
	}{
		{"dir", fuse.S_IFDIR},
		{"symlink", fuse.S_IFLNK},
		{"fifo", fuse.S_IFIFO},
		{"reg", fuse.S_IFREG},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m mockFuseDockerClient
			root := NewNode(&m, "/root", "")
			fusefs.NewNodeFS(root, &fusefs.Options{})
			m.On("stat", mock.Anything, "/root/"+tt.name, mock.Anything).Run(func(args mock.Arguments) {
				attr := args.Get(2).(*statAttr)
				attr.FuseAttr = fuse.Attr{Ino: 1, Mode: tt.mode}
			}).Return(syscall.Errno(0))
			var out fuse.EntryOut
			inode, errno := root.Lookup(context.Background(), tt.name, &out)
			assert.Equal(t, syscall.Errno(0), errno)
			if assert.NotNil(t, inode) {
				assert.Equal(t, tt.mode, inode.Mode())
			}
			m.AssertExpectations(t)
		})
	}
}

func TestNodeLookupError(t *testing.T) {
	var m mockFuseDockerClient
	root := NewNode(&m, "/root", "")
	fusefs.NewNodeFS(root, &fusefs.Options{})
	m.On("stat", mock.Anything, "/root/missing", mock.Anything).Return(syscall.ENOENT)
	var out fuse.EntryOut
	inode, errno := root.Lookup(context.Background(), "missing", &out)
	assert.Nil(t, inode)
	assert.Equal(t, syscall.ENOENT, errno)
	m.AssertExpectations(t)
}
