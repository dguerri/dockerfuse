package server

import (
	"dockerfuse/pkg/rpc_common"
	"fmt"
	"io/fs"
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

func (o *mockFS) Lstat(n string) (fs.FileInfo, error) {
	args := o.Called(n)
	return args.Get(0).(fs.FileInfo), args.Error(1)
}
func (o *mockFS) ReadDir(n string) ([]fs.DirEntry, error) {
	args := o.Called(n)
	mdes := args.Get(0).([]mockDirEntry)
	fsDirEntries := make([]fs.DirEntry, len(mdes))
	for i, v := range mdes {
		fsDirEntries[i] = fs.DirEntry(v)
	}
	return fsDirEntries, args.Error(1)
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

func (fi mockFileInfo) Size() int64        { return fi.size }
func (fi mockFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi mockFileInfo) ModTime() time.Time { return fi.modTime }
func (fi mockFileInfo) Sys() any           { return &fi.sys }
func (fi mockFileInfo) Name() string       { return fi.name }
func (fi mockFileInfo) IsDir() bool        { return fi.Mode().IsDir() }

type mockDirEntry struct {
	name          string
	fileMode      uint32
	fileInfo      mockFileInfo
	fileInfoError error
}

func (de mockDirEntry) Name() string               { return de.name }
func (de mockDirEntry) IsDir() bool                { return de.Type().IsDir() }
func (de mockDirEntry) Type() fs.FileMode          { return fs.FileMode(de.fileMode) }
func (de mockDirEntry) Info() (fs.FileInfo, error) { return de.fileInfo, de.fileInfoError }

func TestStat(t *testing.T) {
	// *** Setup
	mFS := new(mockFS)
	dfFS = mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()
	var (
		mockLstatCall, mockReadlink *mock.Call
		reply                       rpc_common.StatReply
		err                         error
	)

	// *** Testing error on Lstat
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

	// *** Testing happy path on regular file
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

	// *** Testing happy path on link
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

func TestReadDir(t *testing.T) {
	// *** Setup
	mFS := new(mockFS)
	dfFS = mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()
	var (
		mockReadDirCall *mock.Call
		reply           rpc_common.ReadDirReply
		err             error
	)

	// *** Testing error on ReadDir
	mockReadDirCall = mFS.On("ReadDir", "/test/error_on_readdir").Return([]mockDirEntry{}, syscall.ENOENT)

	err = dfFSOps.ReadDir(rpc_common.ReadDirRequest{FullPath: "/test/error_on_readdir"}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)
	mFS.AssertCalled(t, "ReadDir", "/test/error_on_readdir")
	assert.Equal(t, rpc_common.ReadDirReply{}, reply)

	mockReadDirCall.Unset()
	reply = rpc_common.ReadDirReply{}

	// *** Testing happy path
	mockReadDirCall = mFS.On("ReadDir", "/test/happy_path").Return([]mockDirEntry{
		{name: "file1", fileInfo: mockFileInfo{sys: syscall.Stat_t{Ino: 29, Mode: 0660}}},
		{name: "link1", fileInfo: mockFileInfo{sys: syscall.Stat_t{Ino: 30, Mode: 0777}}},
		{name: "dire1", fileInfo: mockFileInfo{sys: syscall.Stat_t{Ino: 31, Mode: 0755}}},
	}, nil)

	err = dfFSOps.ReadDir(rpc_common.ReadDirRequest{FullPath: "/test/happy_path"}, &reply)

	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	mFS.AssertCalled(t, "ReadDir", "/test/happy_path")
	assert.Equal(t, rpc_common.ReadDirReply{
		DirEntries: []rpc_common.DirEntry{
			{Ino: 29, Name: "file1", Mode: 0660},
			{Ino: 30, Name: "link1", Mode: 0777},
			{Ino: 31, Name: "dire1", Mode: 0755},
		},
	}, reply)

	mockReadDirCall.Unset()
	reply = rpc_common.ReadDirReply{}

	// *** Testing empty directory
	mockReadDirCall = mFS.On("ReadDir", "/test/happy_path").Return([]mockDirEntry{}, nil)

	err = dfFSOps.ReadDir(rpc_common.ReadDirRequest{FullPath: "/test/happy_path"}, &reply)

	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	mFS.AssertCalled(t, "ReadDir", "/test/happy_path")
	assert.Equal(t, rpc_common.ReadDirReply{DirEntries: []rpc_common.DirEntry{}}, reply)

	mockReadDirCall.Unset()
	reply = rpc_common.ReadDirReply{}

	// *** Testing ErrNotExist on Info()
	mockReadDirCall = mFS.On("ReadDir", "/test/info_err_no_exist").Return([]mockDirEntry{
		{name: "file1", fileInfo: mockFileInfo{sys: syscall.Stat_t{Ino: 29, Mode: 0660}}, fileInfoError: fs.ErrNotExist},
		{name: "link1", fileInfo: mockFileInfo{sys: syscall.Stat_t{Ino: 30, Mode: 0777}}},
		{name: "dire1", fileInfo: mockFileInfo{sys: syscall.Stat_t{Ino: 31, Mode: 0755}}},
	}, nil)

	err = dfFSOps.ReadDir(rpc_common.ReadDirRequest{FullPath: "/test/info_err_no_exist"}, &reply)

	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	mFS.AssertCalled(t, "ReadDir", "/test/info_err_no_exist")
	assert.Equal(t, rpc_common.ReadDirReply{
		DirEntries: []rpc_common.DirEntry{
			{Ino: 30, Name: "link1", Mode: 0777},
			{Ino: 31, Name: "dire1", Mode: 0755},
		},
	}, reply)

	mockReadDirCall.Unset()
	reply = rpc_common.ReadDirReply{}

	// *** Testing unexpected error on Info()
	mockReadDirCall = mFS.On("ReadDir", "/test/info_err_unexpected").Return([]mockDirEntry{
		{name: "file1", fileInfo: mockFileInfo{sys: syscall.Stat_t{Ino: 29, Mode: 0660}}, fileInfoError: fs.ErrInvalid},
		{name: "link1", fileInfo: mockFileInfo{sys: syscall.Stat_t{Ino: 30, Mode: 0777}}},
		{name: "dire1", fileInfo: mockFileInfo{sys: syscall.Stat_t{Ino: 31, Mode: 0755}}},
	}, nil)

	err = dfFSOps.ReadDir(rpc_common.ReadDirRequest{FullPath: "/test/info_err_unexpected"}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EIO"), err)
	}
	mFS.AssertExpectations(t)
	mFS.AssertCalled(t, "ReadDir", "/test/info_err_unexpected")
	assert.Equal(t, rpc_common.ReadDirReply{DirEntries: []rpc_common.DirEntry{}}, reply)

	mockReadDirCall.Unset()
	reply = rpc_common.ReadDirReply{}

}
