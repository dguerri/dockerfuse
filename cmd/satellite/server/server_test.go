package server

import (
	"dockerfuse/pkg/rpc_common"
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockFS implements mock fileSystem for testing
type mockFS struct {
	mock.Mock
}

func (o *mockFS) Lstat(n string) (os.FileInfo, error) {
	args := o.Called(n)
	return args.Get(0).(os.FileInfo), args.Error(1)
}
func (o *mockFS) ReadDir(n string) ([]os.DirEntry, error) {
	args := o.Called(n)
	return args.Get(0).([]os.DirEntry), args.Error(1)
}
func (o *mockFS) Readlink(n string) (string, error) {
	args := o.Called(n)
	return args.String(0), args.Error(1)
}
func (o *mockFS) OpenFile(n string, f int, p os.FileMode) (file, error) {
	args := o.Called(n, f, p)
	return args.Get(0).(file), args.Error(1)
}

type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	sys     syscall.Stat_t
}

func (fs mockFileInfo) Size() int64        { return fs.size }
func (fs mockFileInfo) Mode() os.FileMode  { return fs.mode }
func (fs mockFileInfo) ModTime() time.Time { return fs.modTime }
func (fs mockFileInfo) Sys() any           { return &fs.sys }
func (fs mockFileInfo) Name() string       { return fs.name }
func (fs mockFileInfo) IsDir() bool        { return fs.Mode().IsDir() }

func TestStat(t *testing.T) {
	// Setup
	mFS := new(mockFS)
	dfFS = mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()
	var (
		mockLstatCall, mockReadlink *mock.Call
		reply                       rpc_common.StatReply
		err                         error
	)

	// Testing error on Lstat
	mockLstatCall = mFS.On("Lstat", "/test/error_on_lstat").Return(mockFileInfo{}, syscall.ENOENT)

	err = dfFSOps.Stat(rpc_common.StatRequest{FullPath: "/test/error_on_lstat"}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)
	mFS.AssertCalled(t, "Lstat", "/test/error_on_lstat")
	mFS.AssertNotCalled(t, "Readlink")
	assert.Equal(t, rpc_common.StatReply{}, reply)

	mockLstatCall.Unset()
	reply = rpc_common.StatReply{}

	// Testing happy path on regular file

	mockLstatCall = mFS.On("Lstat", "/test/reg").Return(mockFileInfo{
		sys: syscall.Stat_t{
			Mode:      0760,
			Nlink:     1,
			Ino:       29,
			Uid:       1,
			Gid:       2,
			Atimespec: syscall.Timespec{Sec: 2929},
			Mtimespec: syscall.Timespec{Sec: 2930},
			Ctimespec: syscall.Timespec{Sec: 2931},
			Blocks:    29,
			Blksize:   1024,
			Size:      29696,
		},
	}, nil)
	mockReadlink = mFS.On("Readlink", "/test/reg").Return("", nil)

	err = dfFSOps.Stat(rpc_common.StatRequest{FullPath: "/test/reg"}, &reply)

	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	mFS.AssertCalled(t, "Lstat", "/test/reg")
	mFS.AssertCalled(t, "Readlink", "/test/reg")
	assert.Equal(t, rpc_common.StatReply{
		Mode:       0760,
		Nlink:      1,
		Ino:        29,
		Uid:        1,
		Gid:        2,
		Atime:      2929,
		Mtime:      2930,
		Ctime:      2931,
		Size:       29696,
		Blocks:     29,
		Blksize:    1024,
		LinkTarget: "",
	}, reply)

	mockLstatCall.Unset()
	mockReadlink.Unset()
	reply = rpc_common.StatReply{}

	// Testing happy path on link

	mockLstatCall = mFS.On("Lstat", "/test/symlink").Return(mockFileInfo{
		sys: syscall.Stat_t{
			Mode:      0777,
			Nlink:     1,
			Ino:       29,
			Uid:       1,
			Gid:       2,
			Atimespec: syscall.Timespec{Sec: 2929},
			Mtimespec: syscall.Timespec{Sec: 2930},
			Ctimespec: syscall.Timespec{Sec: 2931},
			Blocks:    4,
			Blksize:   1024,
			Size:      4096,
		},
	}, nil)
	mockReadlink = mFS.On("Readlink", "/test/symlink").Return("/test/symlinktarget", nil)

	err = dfFSOps.Stat(rpc_common.StatRequest{FullPath: "/test/symlink"}, &reply)

	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	mFS.AssertCalled(t, "Lstat", "/test/symlink")
	mFS.AssertCalled(t, "Readlink", "/test/symlink")
	assert.Equal(t, rpc_common.StatReply{
		Mode:       0777,
		Nlink:      1,
		Ino:        29,
		Uid:        1,
		Gid:        2,
		Atime:      2929,
		Mtime:      2930,
		Ctime:      2931,
		Size:       4096,
		Blocks:     4,
		Blksize:    1024,
		LinkTarget: "/test/symlinktarget",
	}, reply)

	mockLstatCall.Unset()
	mockReadlink.Unset()
	reply = rpc_common.StatReply{}
}
