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
type mockFS struct{ mock.Mock }

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

// mockFileInfo implements mock os.FileInfo for testing
type mockFileInfo struct{ mock.Mock }

func (f mockFileInfo) Size() int64        { a := f.Called(); return a.Get(0).(int64) }
func (f mockFileInfo) Mode() os.FileMode  { a := f.Called(); return a.Get(0).(os.FileMode) }
func (f mockFileInfo) ModTime() time.Time { a := f.Called(); return a.Get(0).(time.Time) }
func (f mockFileInfo) Sys() any           { a := f.Called(); return a.Get(0).(*syscall.Stat_t) }
func (f mockFileInfo) Name() string       { a := f.Called(); return a.String(0) }
func (f mockFileInfo) IsDir() bool        { f.Called(); return f.Mode().IsDir() }

// mockDirEntry implements mock os.DirEntry for testing
type mockDirEntry struct{ mock.Mock }

func (d mockDirEntry) Name() string      { a := d.Called(); return a.String(0) }
func (d mockDirEntry) IsDir() bool       { d.Called(); return d.Type().IsDir() }
func (d mockDirEntry) Type() fs.FileMode { a := d.Called(); return a.Get(0).(fs.FileMode) }
func (d mockDirEntry) Info() (fs.FileInfo, error) {
	a := d.Called()
	return a.Get(0).(mockFileInfo), a.Error(1)
}

// mockMockFile implements mock os.File for testing
type mockFile struct{ mock.Mock }

func (f mockFile) Fd() uintptr                { a := f.Called(); return a.Get(0).(uintptr) }
func (f mockFile) Close() error               { a := f.Called(); return a.Error(0) }
func (f mockFile) Read(p []byte) (int, error) { a := f.Called(p); return a.Int(0), a.Error(0) }
func (f mockFile) ReadAt(p []byte, o int64) (int, error) {
	a := f.Called(p, o)
	return a.Int(0), a.Error(1)
}
func (f mockFile) Seek(o int64, w int) (int64, error) {
	a := f.Called(o, w)
	return a.Get(0).(int64), a.Error(1)
}
func (f mockFile) Write(p []byte) (int, error) { a := f.Called(p); return a.Int(0), a.Error(0) }
func (f mockFile) WriteAt(p []byte, o int64) (int, error) {
	a := f.Called(p, o)
	return a.Int(0), a.Error(1)
}
func (f mockFile) Stat() (os.FileInfo, error) {
	a := f.Called()
	return a.Get(0).(mockFileInfo), a.Error(1)
}
func (f mockFile) Sync() error { a := f.Called(); return a.Error(0) }

func TestStat(t *testing.T) {
	// *** Setup
	mFS := new(mockFS)
	mFI := new(mockFileInfo)
	dfFS = mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()
	var (
		mockOSLstatCall, mockOSReadlink, mockFileInfoSysCall *mock.Call
		reply                                                rpc_common.StatReply
		err                                                  error
	)

	// *** Testing error on Lstat
	mockOSLstatCall = mFS.On("Lstat", "/test/error_on_lstat").Return(mockFileInfo{}, syscall.ENOENT)

	err = dfFSOps.Stat(rpc_common.StatRequest{FullPath: "/test/error_on_lstat"}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)
	mFS.AssertNotCalled(t, "Readlink")
	assert.Equal(t, rpc_common.StatReply{}, reply)

	mockOSLstatCall.Unset()
	reply = rpc_common.StatReply{}

	// *** Testing happy path on regular file
	mockFileInfoSysCall = mFI.On("Sys").Return(&syscall.Stat_t{
		Mode:    0760,
		Nlink:   1,
		Ino:     29,
		Uid:     1,
		Gid:     2,
		Blocks:  29,
		Blksize: 1024,
		Size:    29696,
	})
	mockOSLstatCall = mFS.On("Lstat", "/test/reg").Return(mFI, nil)
	mockOSReadlink = mFS.On("Readlink", "/test/reg").Return("", nil)

	err = dfFSOps.Stat(rpc_common.StatRequest{FullPath: "/test/reg"}, &reply)

	assert.NoError(t, err)
	mFI.AssertExpectations(t)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.StatReply{
		Mode:       0760,
		Nlink:      1,
		Ino:        29,
		Uid:        1,
		Gid:        2,
		Size:       29696,
		Blocks:     29,
		Blksize:    1024,
		LinkTarget: "",
	}, reply)

	mockFileInfoSysCall.Unset()
	mockOSLstatCall.Unset()
	mockOSReadlink.Unset()
	reply = rpc_common.StatReply{}

	// *** Testing happy path on link
	mockFileInfoSysCall = mFI.On("Sys").Return(&syscall.Stat_t{
		Mode:    0777,
		Nlink:   1,
		Ino:     29,
		Uid:     1,
		Gid:     2,
		Blocks:  4,
		Blksize: 1024,
		Size:    4096,
	})
	mockOSLstatCall = mFS.On("Lstat", "/test/symlink").Return(mFI, nil)
	mockOSReadlink = mFS.On("Readlink", "/test/symlink").Return("/test/symlinktarget", nil)

	err = dfFSOps.Stat(rpc_common.StatRequest{FullPath: "/test/symlink"}, &reply)

	assert.NoError(t, err)
	mFI.AssertExpectations(t)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.StatReply{
		Mode:       0777,
		Nlink:      1,
		Ino:        29,
		Uid:        1,
		Gid:        2,
		Size:       4096,
		Blocks:     4,
		Blksize:    1024,
		LinkTarget: "/test/symlinktarget",
	}, reply)

	mockFileInfoSysCall.Unset()
	mockOSLstatCall.Unset()
	mockOSReadlink.Unset()
	reply = rpc_common.StatReply{}
}

func TestReadDir(t *testing.T) {
	// *** Setup
	mFS := new(mockFS)
	dfFS = mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()
	var (
		mockOSReadDirCall *mock.Call
		reply             rpc_common.ReadDirReply
		err               error
		mFIs              []mockFileInfo
		mDIs              []mockDirEntry
	)
	mFIs = []mockFileInfo{{}, {}, {}}
	mDIs = []mockDirEntry{{}, {}, {}}

	// *** Testing error on ReadDir
	mockOSReadDirCall = mFS.On("ReadDir", "/test/error_on_readdir").Return([]mockDirEntry{}, syscall.ENOENT)

	err = dfFSOps.ReadDir(rpc_common.ReadDirRequest{FullPath: "/test/error_on_readdir"}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.ReadDirReply{}, reply)

	mockOSReadDirCall.Unset()
	reply = rpc_common.ReadDirReply{}

	// *** Testing happy path
	mFIs[0].On("Sys").Return(&syscall.Stat_t{Ino: 29, Mode: 0660})
	mDIs[0].On("Name").Return("file1")
	mDIs[0].On("Info").Return(mFIs[0], nil)
	mFIs[1].On("Sys").Return(&syscall.Stat_t{Ino: 30, Mode: 0777})
	mDIs[1].On("Name").Return("link1")
	mDIs[1].On("Info").Return(mFIs[1], nil)
	mFIs[2].On("Sys").Return(&syscall.Stat_t{Ino: 31, Mode: 0755})
	mDIs[2].On("Name").Return("dir1")
	mDIs[2].On("Info").Return(mFIs[2], nil)
	mockOSReadDirCall = mFS.On("ReadDir", "/test/happy_path").Return(mDIs, nil)

	err = dfFSOps.ReadDir(rpc_common.ReadDirRequest{FullPath: "/test/happy_path"}, &reply)

	assert.NoError(t, err)
	for i := range mDIs {
		mDIs[i].AssertExpectations(t)
		mFIs[i].AssertExpectations(t)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.ReadDirReply{
		DirEntries: []rpc_common.DirEntry{
			{Ino: 29, Name: "file1", Mode: 0660},
			{Ino: 30, Name: "link1", Mode: 0777},
			{Ino: 31, Name: "dir1", Mode: 0755},
		},
	}, reply)

	mockOSReadDirCall.Unset()
	for i := range mFIs {
		for _, e := range mFIs[i].ExpectedCalls {
			e.Unset()
		}
		for _, e := range mDIs[i].ExpectedCalls {
			e.Unset()
		}
	}
	reply = rpc_common.ReadDirReply{}

	// *** Testing empty directory
	mockOSReadDirCall = mFS.On("ReadDir", "/test/happy_path").Return([]mockDirEntry{}, nil)

	err = dfFSOps.ReadDir(rpc_common.ReadDirRequest{FullPath: "/test/happy_path"}, &reply)

	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.ReadDirReply{DirEntries: []rpc_common.DirEntry{}}, reply)

	mockOSReadDirCall.Unset()
	for i := range mFIs {
		for _, e := range mFIs[i].ExpectedCalls {
			e.Unset()
		}
		for _, e := range mDIs[i].ExpectedCalls {
			e.Unset()
		}
	}
	reply = rpc_common.ReadDirReply{}

	// *** Testing ErrNotExist on Info()
	mFIs[0].On("Sys").Return(&syscall.Stat_t{Ino: 29, Mode: 0660})
	mDIs[0].On("Name").Return("file1")
	mDIs[0].On("Info").Return(mockFileInfo{}, fs.ErrNotExist)
	mFIs[1].On("Sys").Return(&syscall.Stat_t{Ino: 30, Mode: 0777})
	mDIs[1].On("Name").Return("link1")
	mDIs[1].On("Info").Return(mFIs[1], nil)
	mFIs[2].On("Sys").Return(&syscall.Stat_t{Ino: 31, Mode: 0755})
	mDIs[2].On("Name").Return("dir1")
	mDIs[2].On("Info").Return(mFIs[2], nil)
	mockOSReadDirCall = mFS.On("ReadDir", "/test/info_err_no_exist").Return(mDIs, nil)

	err = dfFSOps.ReadDir(rpc_common.ReadDirRequest{FullPath: "/test/info_err_no_exist"}, &reply)

	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.ReadDirReply{
		DirEntries: []rpc_common.DirEntry{
			{Ino: 30, Name: "link1", Mode: 0777},
			{Ino: 31, Name: "dir1", Mode: 0755},
		},
	}, reply)

	mockOSReadDirCall.Unset()
	for i := range mFIs {
		for _, e := range mFIs[i].ExpectedCalls {
			e.Unset()
		}
		for _, e := range mDIs[i].ExpectedCalls {
			e.Unset()
		}
	}
	reply = rpc_common.ReadDirReply{}

	// *** Testing unexpected error on Info()
	mFIs[0].On("Sys").Return(&syscall.Stat_t{Ino: 29, Mode: 0660})
	mDIs[0].On("Name").Return("file1")
	mDIs[0].On("Info").Return(mockFileInfo{}, syscall.EINVAL)
	mFIs[1].On("Sys").Return(&syscall.Stat_t{Ino: 30, Mode: 0777})
	mDIs[1].On("Name").Return("link1")
	mDIs[1].On("Info").Return(mFIs[1], nil)
	mFIs[2].On("Sys").Return(&syscall.Stat_t{Ino: 31, Mode: 0755})
	mDIs[2].On("Name").Return("dir1")
	mDIs[2].On("Info").Return(mFIs[2], nil)
	mockOSReadDirCall = mFS.On("ReadDir", "/test/info_err_unexpected").Return(mDIs, nil)

	err = dfFSOps.ReadDir(rpc_common.ReadDirRequest{FullPath: "/test/info_err_unexpected"}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EIO"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.ReadDirReply{DirEntries: []rpc_common.DirEntry{}}, reply)

	mockOSReadDirCall.Unset()
	reply = rpc_common.ReadDirReply{}
}

func TestOpen(t *testing.T) {
	// *** Setup
	mFS := new(mockFS)
	dfFS = mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()
	var (
		mockOSOpenFileCall,
		mockOSReadlinkCall,
		mockFileFdCall,
		mockFileStatCall,
		mockFileInfoSysCall,
		mockFileCloseCall *mock.Call
		reply rpc_common.OpenReply
		err   error
	)
	mFile := new(mockFile)
	mFileInfo := new(mockFileInfo)

	// *** Testing error on OpenFile
	mockOSOpenFileCall = mFS.On("OpenFile",
		"/test/error_on_openfile",
		syscall.O_CREAT|syscall.O_RDWR,
		fs.FileMode(0666),
	).Return(mockFile{}, syscall.ENOENT)

	err = dfFSOps.Open(rpc_common.OpenRequest{
		FullPath: "/test/error_on_openfile",
		SAFlags:  rpc_common.SystemToSAFlags(syscall.O_CREAT | syscall.O_RDWR),
		Mode:     fs.FileMode(0666),
	}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.OpenReply{}, reply)

	mockOSOpenFileCall.Unset()
	reply = rpc_common.OpenReply{}

	// *** Testing Open on a regular existing file
	mockFileInfoSysCall = mFileInfo.On("Sys").Return(&syscall.Stat_t{
		Mode:    0660,
		Nlink:   2,
		Ino:     29,
		Uid:     1,
		Gid:     2,
		Size:    3072,
		Blocks:  3,
		Blksize: 1024,
	})
	mockOSOpenFileCall = mFS.On("OpenFile", "/test/openfile_reg", syscall.O_RDWR, fs.FileMode(0640)).Return(mFile, nil)
	mockFileFdCall = mFile.On("Fd").Return(uintptr(29))
	mockFileStatCall = mFile.On("Stat").Return(*mFileInfo, nil)
	mockOSReadlinkCall = mFS.On("Readlink", "/test/openfile_reg").Return("", nil)

	err = dfFSOps.Open(rpc_common.OpenRequest{
		FullPath: "/test/openfile_reg",
		SAFlags:  rpc_common.SystemToSAFlags(syscall.O_RDWR),
		Mode:     fs.FileMode(0640),
	}, &reply)

	assert.NoError(t, err)
	mFileInfo.AssertExpectations(t)
	mFile.AssertExpectations(t)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.OpenReply{
		FD: 29,
		StatReply: rpc_common.StatReply{
			Mode:       0660,
			Nlink:      2,
			Ino:        29,
			Uid:        1,
			Gid:        2,
			Size:       3072,
			Blocks:     3,
			Blksize:    1024,
			LinkTarget: "",
		},
	}, reply)

	mockOSOpenFileCall.Unset()
	mockOSReadlinkCall.Unset()
	mockFileFdCall.Unset()
	mockFileStatCall.Unset()
	mockFileInfoSysCall.Unset()
	reply = rpc_common.OpenReply{}

	// *** Testing Open on a symlink, closing the previous Fd
	mockFileInfoSysCall = mFileInfo.On("Sys").Return(&syscall.Stat_t{
		Mode:    0777,
		Nlink:   1,
		Ino:     29,
		Uid:     1,
		Gid:     2,
		Size:    1024,
		Blocks:  1,
		Blksize: 1024,
	})
	mockOSOpenFileCall = mFS.On("OpenFile", "/test/openfile_symlink", syscall.O_RDWR, fs.FileMode(0640)).Return(mFile, nil)
	mockFileFdCall = mFile.On("Fd").Return(uintptr(29))
	mockFileCloseCall = mFile.On("Close").Return(nil) // FD 29 was already in the table since last test
	mockFileStatCall = mFile.On("Stat").Return(*mFileInfo, nil)
	mockOSReadlinkCall = mFS.On("Readlink", "/test/openfile_symlink").Return("/test/openfile_symlink_target", nil)

	err = dfFSOps.Open(rpc_common.OpenRequest{
		FullPath: "/test/openfile_symlink",
		SAFlags:  rpc_common.SystemToSAFlags(syscall.O_RDWR),
		Mode:     fs.FileMode(0640),
	}, &reply)

	assert.NoError(t, err)
	mFileInfo.AssertExpectations(t)
	mFile.AssertExpectations(t)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.OpenReply{
		FD: 29,
		StatReply: rpc_common.StatReply{
			Mode:       0777,
			Nlink:      1,
			Ino:        29,
			Uid:        1,
			Gid:        2,
			Size:       1024,
			Blocks:     1,
			Blksize:    1024,
			LinkTarget: "/test/openfile_symlink_target",
		},
	}, reply)

	mockOSOpenFileCall.Unset()
	mockOSReadlinkCall.Unset()
	mockFileFdCall.Unset()
	mockFileStatCall.Unset()
	mockFileInfoSysCall.Unset()
	mockFileCloseCall.Unset()
	reply = rpc_common.OpenReply{}

	// *** Testing Open on a regular existing file, w/ readlink error
	mockFileInfoSysCall = mFileInfo.On("Sys").Return(&syscall.Stat_t{
		Mode:    0660,
		Nlink:   2,
		Ino:     29,
		Uid:     1,
		Gid:     2,
		Size:    3072,
		Blocks:  3,
		Blksize: 1024,
	})
	mockOSOpenFileCall = mFS.On("OpenFile", "/test/openfile_reg", syscall.O_RDWR, fs.FileMode(0640)).Return(mFile, nil)
	mockFileFdCall = mFile.On("Fd").Return(uintptr(30))
	mockFileStatCall = mFile.On("Stat").Return(*mFileInfo, nil)
	mockOSReadlinkCall = mFS.On("Readlink", "/test/openfile_reg").Return("", syscall.EINVAL)

	err = dfFSOps.Open(rpc_common.OpenRequest{
		FullPath: "/test/openfile_reg",
		SAFlags:  rpc_common.SystemToSAFlags(syscall.O_RDWR),
		Mode:     fs.FileMode(0640),
	}, &reply)

	assert.NoError(t, err)
	mFileInfo.AssertExpectations(t)
	mFile.AssertExpectations(t)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.OpenReply{
		FD: 30,
		StatReply: rpc_common.StatReply{
			Mode:       0660,
			Nlink:      2,
			Ino:        29,
			Uid:        1,
			Gid:        2,
			Size:       3072,
			Blocks:     3,
			Blksize:    1024,
			LinkTarget: "",
		},
	}, reply)

	mockOSOpenFileCall.Unset()
	mockOSReadlinkCall.Unset()
	mockFileFdCall.Unset()
	mockFileStatCall.Unset()
	mockFileInfoSysCall.Unset()
	reply = rpc_common.OpenReply{}
}

func TestClose(t *testing.T) {
	// *** Setup
	mFS := new(mockFS)
	dfFS = mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()
	var (
		mockFileCloseCall *mock.Call
		reply             rpc_common.CloseReply
		err               error
	)
	mFile := new(mockFile)

	// *** Testing error on Close
	mockFileCloseCall = mFile.On("Close").Return(syscall.EACCES)

	dfFSOps.fds[29] = mFile
	err = dfFSOps.Close(rpc_common.CloseRequest{FD: 29}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EACCES"), err)
	}
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.CloseReply{}, reply)

	mockFileCloseCall.Unset()
	reply = rpc_common.CloseReply{}

	// *** Testing invalid FD
	mockFileCloseCall = mFile.On("Close").Return(nil)

	delete(dfFSOps.fds, 29)
	err = dfFSOps.Close(rpc_common.CloseRequest{FD: 29}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFile.AssertNotCalled(t, "Close")
	assert.Equal(t, rpc_common.CloseReply{}, reply)

	mockFileCloseCall.Unset()
	reply = rpc_common.CloseReply{}

	// *** Testing happy path
	mockFileCloseCall = mFile.On("Close").Return(nil)

	dfFSOps.fds[29] = mFile
	err = dfFSOps.Close(rpc_common.CloseRequest{FD: 29}, &reply)

	assert.NoError(t, err)
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.CloseReply{}, reply)

	mockFileCloseCall.Unset()
	reply = rpc_common.CloseReply{}
}

func TestRead(t *testing.T) {
	// *** Setup
	mFS := new(mockFS)
	dfFS = mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()
	var (
		mockFileReadAtCall *mock.Call
		reply              rpc_common.ReadReply
		err                error
	)
	mFile := new(mockFile)

	// *** Testing error on ReadAt
	mockFileReadAtCall = mFile.On("ReadAt", make([]byte, 10), int64(0)).Return(0, syscall.EACCES)

	dfFSOps.fds[29] = mFile
	err = dfFSOps.Read(rpc_common.ReadRequest{FD: 29, Offset: 0, Num: 10}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EACCES"), err)
	}
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.ReadReply{}, reply)

	mockFileReadAtCall.Unset()
	reply = rpc_common.ReadReply{}

	// *** Testing invalid FD
	mockFileReadAtCall = mFile.On("ReadAt", make([]byte, 10), int64(0)).Return(10, nil)

	delete(dfFSOps.fds, 29)
	err = dfFSOps.Read(rpc_common.ReadRequest{FD: 29, Offset: 0, Num: 10}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFile.AssertNotCalled(t, "ReadAt", mock.Anything, mock.Anything)
	assert.Equal(t, rpc_common.ReadReply{}, reply)

	mockFileReadAtCall.Unset()
	reply = rpc_common.ReadReply{}

	// *** Testing happy path on ReadAt
	mockFileReadAtCall = mFile.On("ReadAt", make([]byte, 5), int64(3)).Return(5, nil).Run(
		func(args mock.Arguments) {
			data := args.Get(0).([]byte)
			num := args.Get(1).(int64)
			for i := 0; i < len(data); i++ {
				data[i] = byte(i + int(num))
			}
		},
	)

	dfFSOps.fds[29] = mFile
	err = dfFSOps.Read(rpc_common.ReadRequest{FD: 29, Offset: 3, Num: 5}, &reply)

	assert.NoError(t, err)
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.ReadReply{Data: []byte{3, 4, 5, 6, 7}}, reply)

	mockFileReadAtCall.Unset()
	reply = rpc_common.ReadReply{}
}

func TestSeek(t *testing.T) {
	// *** Setup
	mFS := new(mockFS)
	dfFS = mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()
	var (
		mockFileSeekCall *mock.Call
		reply            rpc_common.SeekReply
		err              error
	)
	mFile := new(mockFile)

	// *** Testing error on Close
	mockFileSeekCall = mFile.On("Seek", int64(10), 0).Return(int64(0), syscall.EACCES)

	dfFSOps.fds[29] = mFile
	err = dfFSOps.Seek(rpc_common.SeekRequest{FD: 29, Offset: 10, Whence: 0}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EACCES"), err)
	}
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.SeekReply{}, reply)

	mockFileSeekCall.Unset()
	reply = rpc_common.SeekReply{}

	// *** Testing invalid FD
	mockFileSeekCall = mFile.On("Seek", int64(0), 0).Return(int64(0), nil)

	delete(dfFSOps.fds, 29)
	err = dfFSOps.Seek(rpc_common.SeekRequest{FD: 29, Offset: 0, Whence: 0}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFile.AssertNotCalled(t, "Seek", mock.Anything, mock.Anything)
	assert.Equal(t, rpc_common.SeekReply{}, reply)

	mockFileSeekCall.Unset()
	reply = rpc_common.SeekReply{}

	// *** Testing happy path for Seek
	mockFileSeekCall = mFile.On("Seek", int64(10), 0).Return(int64(10), nil)

	dfFSOps.fds[29] = mFile
	err = dfFSOps.Seek(rpc_common.SeekRequest{FD: 29, Offset: 10, Whence: 0}, &reply)

	assert.NoError(t, err)
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.SeekReply{Num: 10}, reply)

	mockFileSeekCall.Unset()
	reply = rpc_common.SeekReply{}
}

func TestWrite(t *testing.T) {
	// *** Setup
	mFS := new(mockFS)
	dfFS = mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()
	var (
		mockFileWriteAtCall *mock.Call
		reply               rpc_common.WriteReply
		err                 error
	)
	mFile := new(mockFile)

	// *** Testing error on WriteAt
	data := []byte{29, 30, 31, 21}
	mockFileWriteAtCall = mFile.On("WriteAt", data, int64(0)).Return(0, syscall.EACCES)

	dfFSOps.fds[29] = mFile
	err = dfFSOps.Write(rpc_common.WriteRequest{FD: 29, Offset: 0, Data: data}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EACCES"), err)
	}
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.WriteReply{}, reply)

	mockFileWriteAtCall.Unset()
	reply = rpc_common.WriteReply{}

	// *** Testing invalid FD
	mockFileWriteAtCall = mFile.On("WriteAt", data, int64(0)).Return(10, nil)

	delete(dfFSOps.fds, 29)
	err = dfFSOps.Write(rpc_common.WriteRequest{FD: 29, Offset: 0, Data: data}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFile.AssertNotCalled(t, "WriteAt", mock.Anything, mock.Anything)
	assert.Equal(t, rpc_common.WriteReply{}, reply)

	mockFileWriteAtCall.Unset()
	reply = rpc_common.WriteReply{}

	// *** Testing happy path on WriteAt
	mockFileWriteAtCall = mFile.On("WriteAt", data, int64(3)).Return(len(data), nil)

	dfFSOps.fds[29] = mFile
	err = dfFSOps.Write(rpc_common.WriteRequest{FD: 29, Offset: 3, Data: data}, &reply)

	assert.NoError(t, err)
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.WriteReply{Num: len(data)}, reply)

	mockFileWriteAtCall.Unset()
	reply = rpc_common.WriteReply{}
}

func TestFsync(t *testing.T) {
	// *** Setup
	mFS := new(mockFS)
	dfFS = mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()
	var (
		mockFileFsyncCall *mock.Call
		reply             rpc_common.FsyncReply
		err               error
	)
	mFile := new(mockFile)

	// *** Testing error on Fsync
	mockFileFsyncCall = mFile.On("Sync").Return(syscall.EACCES)

	dfFSOps.fds[29] = mFile
	err = dfFSOps.Fsync(rpc_common.FsyncRequest{FD: 29}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EACCES"), err)
	}
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.FsyncReply{}, reply)

	mockFileFsyncCall.Unset()
	reply = rpc_common.FsyncReply{}

	// *** Testing invalid FD
	mockFileFsyncCall = mFile.On("Sync").Return(nil)

	delete(dfFSOps.fds, 29)
	err = dfFSOps.Fsync(rpc_common.FsyncRequest{FD: 29}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFile.AssertNotCalled(t, "Sync")
	assert.Equal(t, rpc_common.FsyncReply{}, reply)

	mockFileFsyncCall.Unset()
	reply = rpc_common.FsyncReply{}

	// *** Testing happy path
	mockFileFsyncCall = mFile.On("Sync").Return(nil)

	dfFSOps.fds[29] = mFile
	err = dfFSOps.Fsync(rpc_common.FsyncRequest{FD: 29}, &reply)

	assert.NoError(t, err)
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.FsyncReply{}, reply)

	mockFileFsyncCall.Unset()
	reply = rpc_common.FsyncReply{}
}
