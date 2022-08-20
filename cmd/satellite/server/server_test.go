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
func (o *mockFS) Remove(n string) error               { args := o.Called(n); return args.Error(0) }
func (o *mockFS) Mkdir(n string, p os.FileMode) error { args := o.Called(n); return args.Error(0) }

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
	dfFSOps := NewDockerFuseFSOps()
	var (
		mFS   mockFS
		mFI   mockFileInfo
		mFile mockFile
		reply rpc_common.StatReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem

	// *** Testing error on Lstat
	mFS = mockFS{}
	mFI = mockFileInfo{}
	reply = rpc_common.StatReply{}
	mFS.On("Lstat", "/test/error_on_lstat").Return(mockFileInfo{}, syscall.ENOENT)

	err = dfFSOps.Stat(rpc_common.StatRequest{FullPath: "/test/error_on_lstat", UseFD: false}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)
	mFS.AssertNotCalled(t, "Readlink")
	assert.Equal(t, rpc_common.StatReply{}, reply)

	// *** Testing happy path on regular file
	mFS = mockFS{}
	mFI = mockFileInfo{}
	reply = rpc_common.StatReply{}
	mFI.On("Sys").Return(&syscall.Stat_t{
		Mode:    0760,
		Nlink:   1,
		Ino:     29,
		Uid:     1,
		Gid:     2,
		Blocks:  29,
		Blksize: 1024,
		Size:    29696,
	})
	mFS.On("Lstat", "/test/reg").Return(mFI, nil)
	mFS.On("Readlink", "/test/reg").Return("", nil)

	err = dfFSOps.Stat(rpc_common.StatRequest{FullPath: "/test/reg", UseFD: false}, &reply)

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

	// *** Testing happy path on link
	mFS = mockFS{}
	mFI = mockFileInfo{}
	reply = rpc_common.StatReply{}
	mFI.On("Sys").Return(&syscall.Stat_t{
		Mode:    0777,
		Nlink:   1,
		Ino:     29,
		Uid:     1,
		Gid:     2,
		Blocks:  4,
		Blksize: 1024,
		Size:    4096,
	})
	mFS.On("Lstat", "/test/symlink").Return(mFI, nil)
	mFS.On("Readlink", "/test/symlink").Return("/test/symlinktarget", nil)

	err = dfFSOps.Stat(rpc_common.StatRequest{FullPath: "/test/symlink", UseFD: false}, &reply)

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

	// *** Testing happy path on regular file, with FD
	mFS = mockFS{}
	mFI = mockFileInfo{}
	mFile = mockFile{}
	reply = rpc_common.StatReply{}
	mFI.On("Sys").Return(&syscall.Stat_t{
		Mode:    0760,
		Nlink:   1,
		Ino:     29,
		Uid:     1,
		Gid:     2,
		Blocks:  29,
		Blksize: 1024,
		Size:    29696,
	})
	mFile.On("Stat").Return(mFI, nil)
	mFS.On("Lstat", "/test/reg").Return(mFI, nil)
	mFS.On("Readlink", "/test/reg").Return("", nil)

	dfFSOps.fds[29] = mFile
	err = dfFSOps.Stat(rpc_common.StatRequest{FullPath: "/test/reg", FD: 29, UseFD: true}, &reply)

	assert.NoError(t, err)
	mFI.AssertExpectations(t)
	mFile.AssertExpectations(t)

	mFS.AssertNotCalled(t, "Stat", mock.Anything)
	mFS.AssertNotCalled(t, "Readlink", mock.Anything)
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
}

func TestReadDir(t *testing.T) {
	// *** Setup
	dfFSOps := NewDockerFuseFSOps()
	var (
		mFS   mockFS
		reply rpc_common.ReadDirReply
		err   error
		mFIs  []mockFileInfo
		mDIs  []mockDirEntry
	)
	dfFS = &mFS // Set mock filesystem
	mFIs = []mockFileInfo{{}, {}, {}}
	mDIs = []mockDirEntry{{}, {}, {}}

	// *** Testing error on ReadDir
	mFS = mockFS{}
	reply = rpc_common.ReadDirReply{}
	mFS.On("ReadDir", "/test/error_on_readdir").Return([]mockDirEntry{}, syscall.ENOENT)

	err = dfFSOps.ReadDir(rpc_common.ReadDirRequest{FullPath: "/test/error_on_readdir"}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.ReadDirReply{}, reply)

	// *** Testing happy path
	mFS = mockFS{}
	reply = rpc_common.ReadDirReply{}
	mFIs[0].On("Sys").Return(&syscall.Stat_t{Ino: 29, Mode: 0660})
	mDIs[0].On("Name").Return("file1")
	mDIs[0].On("Info").Return(mFIs[0], nil)
	mFIs[1].On("Sys").Return(&syscall.Stat_t{Ino: 30, Mode: 0777})
	mDIs[1].On("Name").Return("link1")
	mDIs[1].On("Info").Return(mFIs[1], nil)
	mFIs[2].On("Sys").Return(&syscall.Stat_t{Ino: 31, Mode: 0755})
	mDIs[2].On("Name").Return("dir1")
	mDIs[2].On("Info").Return(mFIs[2], nil)
	mFS.On("ReadDir", "/test/happy_path").Return(mDIs, nil)

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
	for i := range mFIs {
		for _, e := range mFIs[i].ExpectedCalls {
			e.Unset()
		}
		for _, e := range mDIs[i].ExpectedCalls {
			e.Unset()
		}
	}

	// *** Testing empty directory
	mFS = mockFS{}
	reply = rpc_common.ReadDirReply{}
	mFS.On("ReadDir", "/test/happy_path").Return([]mockDirEntry{}, nil)

	err = dfFSOps.ReadDir(rpc_common.ReadDirRequest{FullPath: "/test/happy_path"}, &reply)

	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.ReadDirReply{DirEntries: []rpc_common.DirEntry{}}, reply)
	for i := range mFIs {
		for _, e := range mFIs[i].ExpectedCalls {
			e.Unset()
		}
		for _, e := range mDIs[i].ExpectedCalls {
			e.Unset()
		}
	}

	// *** Testing ErrNotExist on Info()
	mFS = mockFS{}
	reply = rpc_common.ReadDirReply{}
	mFIs[0].On("Sys").Return(&syscall.Stat_t{Ino: 29, Mode: 0660})
	mDIs[0].On("Name").Return("file1")
	mDIs[0].On("Info").Return(mockFileInfo{}, fs.ErrNotExist)
	mFIs[1].On("Sys").Return(&syscall.Stat_t{Ino: 30, Mode: 0777})
	mDIs[1].On("Name").Return("link1")
	mDIs[1].On("Info").Return(mFIs[1], nil)
	mFIs[2].On("Sys").Return(&syscall.Stat_t{Ino: 31, Mode: 0755})
	mDIs[2].On("Name").Return("dir1")
	mDIs[2].On("Info").Return(mFIs[2], nil)
	mFS.On("ReadDir", "/test/info_err_no_exist").Return(mDIs, nil)

	err = dfFSOps.ReadDir(rpc_common.ReadDirRequest{FullPath: "/test/info_err_no_exist"}, &reply)

	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.ReadDirReply{
		DirEntries: []rpc_common.DirEntry{
			{Ino: 30, Name: "link1", Mode: 0777},
			{Ino: 31, Name: "dir1", Mode: 0755},
		},
	}, reply)
	for i := range mFIs {
		for _, e := range mFIs[i].ExpectedCalls {
			e.Unset()
		}
		for _, e := range mDIs[i].ExpectedCalls {
			e.Unset()
		}
	}

	// *** Testing unexpected error on Info()
	mFS = mockFS{}
	reply = rpc_common.ReadDirReply{}
	mFIs[0].On("Sys").Return(&syscall.Stat_t{Ino: 29, Mode: 0660})
	mDIs[0].On("Name").Return("file1")
	mDIs[0].On("Info").Return(mockFileInfo{}, syscall.EINVAL)
	mFIs[1].On("Sys").Return(&syscall.Stat_t{Ino: 30, Mode: 0777})
	mDIs[1].On("Name").Return("link1")
	mDIs[1].On("Info").Return(mFIs[1], nil)
	mFIs[2].On("Sys").Return(&syscall.Stat_t{Ino: 31, Mode: 0755})
	mDIs[2].On("Name").Return("dir1")
	mDIs[2].On("Info").Return(mFIs[2], nil)
	mFS.On("ReadDir", "/test/info_err_unexpected").Return(mDIs, nil)

	err = dfFSOps.ReadDir(rpc_common.ReadDirRequest{FullPath: "/test/info_err_unexpected"}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EIO"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.ReadDirReply{DirEntries: []rpc_common.DirEntry{}}, reply)
}

func TestOpen(t *testing.T) {
	// *** Setup
	var (
		mFS   mockFS
		mFI   mockFileInfo
		mFile mockFile
		reply rpc_common.OpenReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()

	// *** Testing error on OpenFile
	mFS = mockFS{}
	dfFSOps.fds = map[uintptr]file{}
	mFS.On("OpenFile", "/test/error_on_openfile", syscall.O_CREAT|syscall.O_RDWR, fs.FileMode(0666)).Return(mockFile{}, syscall.ENOENT)

	reply = rpc_common.OpenReply{}
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

	// *** Testing Open on a regular existing file
	mFS = mockFS{}
	mFile = mockFile{}
	mFI = mockFileInfo{}
	dfFSOps.fds = map[uintptr]file{}
	mFI.On("Sys").Return(&syscall.Stat_t{
		Mode:    0660,
		Nlink:   2,
		Ino:     29,
		Uid:     1,
		Gid:     2,
		Size:    3072,
		Blocks:  3,
		Blksize: 1024,
	})
	mFile.On("Fd").Return(uintptr(29))
	mFile.On("Stat").Return(mFI, nil)
	mFS.On("OpenFile", "/test/openfile_reg", syscall.O_RDWR, fs.FileMode(0640)).Return(mFile, nil)
	mFS.On("Readlink", "/test/openfile_reg").Return("", nil)

	reply = rpc_common.OpenReply{}
	err = dfFSOps.Open(rpc_common.OpenRequest{
		FullPath: "/test/openfile_reg",
		SAFlags:  rpc_common.SystemToSAFlags(syscall.O_RDWR),
		Mode:     fs.FileMode(0640),
	}, &reply)

	assert.NoError(t, err)
	mFI.AssertExpectations(t)
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

	// *** Testing Open on a symlink, closing the previous Fd
	mFS = mockFS{}
	mFile = mockFile{}
	mFI = mockFileInfo{}
	mFI.On("Sys").Return(&syscall.Stat_t{
		Mode:    0777,
		Nlink:   1,
		Ino:     29,
		Uid:     1,
		Gid:     2,
		Size:    1024,
		Blocks:  1,
		Blksize: 1024,
	})
	mFile.On("Fd").Return(uintptr(29))
	mFile.On("Close").Return(nil)
	mFile.On("Stat").Return(mFI, nil)
	mFS.On("OpenFile", "/test/openfile_symlink", syscall.O_RDWR, fs.FileMode(0640)).Return(mFile, nil)
	mFS.On("Readlink", "/test/openfile_symlink").Return("/test/openfile_symlink_target", nil)
	dfFSOps.fds = map[uintptr]file{29: mFile}

	reply = rpc_common.OpenReply{}
	err = dfFSOps.Open(rpc_common.OpenRequest{
		FullPath: "/test/openfile_symlink",
		SAFlags:  rpc_common.SystemToSAFlags(syscall.O_RDWR),
		Mode:     fs.FileMode(0640),
	}, &reply)

	assert.NoError(t, err)
	mFI.AssertExpectations(t)
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

	// *** Testing Open on a regular existing file, w/ readlink error
	mFS = mockFS{}
	mFile = mockFile{}
	mFI = mockFileInfo{}
	dfFSOps.fds = map[uintptr]file{}
	mFI.On("Sys").Return(&syscall.Stat_t{
		Mode:    0660,
		Nlink:   2,
		Ino:     29,
		Uid:     1,
		Gid:     2,
		Size:    3072,
		Blocks:  3,
		Blksize: 1024,
	})
	mFile.On("Fd").Return(uintptr(30))
	mFile.On("Stat").Return(mFI, nil)
	mFS.On("OpenFile", "/test/openfile_reg", syscall.O_RDWR, fs.FileMode(0640)).Return(mFile, nil)
	mFS.On("Readlink", "/test/openfile_reg").Return("", syscall.EINVAL)

	reply = rpc_common.OpenReply{}
	err = dfFSOps.Open(rpc_common.OpenRequest{
		FullPath: "/test/openfile_reg",
		SAFlags:  rpc_common.SystemToSAFlags(syscall.O_RDWR),
		Mode:     fs.FileMode(0640),
	}, &reply)

	assert.NoError(t, err)
	mFI.AssertExpectations(t)
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
}

func TestClose(t *testing.T) {
	// *** Setup
	dfFSOps := NewDockerFuseFSOps()
	var (
		mFS   mockFS
		mFile mockFile
		reply rpc_common.CloseReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem

	// *** Testing error on Close
	mFS = mockFS{}
	mFile = mockFile{}
	reply = rpc_common.CloseReply{}
	mFile.On("Close").Return(syscall.EACCES)
	dfFSOps.fds = map[uintptr]file{29: mFile}

	err = dfFSOps.Close(rpc_common.CloseRequest{FD: 29}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EACCES"), err)
	}
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.CloseReply{}, reply)

	// *** Testing invalid FD
	mFS = mockFS{}
	mFile = mockFile{}
	reply = rpc_common.CloseReply{}
	mFile.On("Close").Return(nil)
	dfFSOps.fds = map[uintptr]file{30: mFile}

	err = dfFSOps.Close(rpc_common.CloseRequest{FD: 29}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFile.AssertNotCalled(t, "Close")
	assert.Equal(t, rpc_common.CloseReply{}, reply)

	// *** Testing happy path
	mFS = mockFS{}
	mFile = mockFile{}
	reply = rpc_common.CloseReply{}
	mFile.On("Close").Return(nil)
	dfFSOps.fds = map[uintptr]file{29: mFile}

	err = dfFSOps.Close(rpc_common.CloseRequest{FD: 29}, &reply)

	assert.NoError(t, err)
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.CloseReply{}, reply)
}

func TestRead(t *testing.T) {
	// *** Setup
	dfFSOps := NewDockerFuseFSOps()
	var (
		mFS   mockFS
		mFile mockFile
		reply rpc_common.ReadReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem

	// *** Testing error on ReadAt
	mFS = mockFS{}
	mFile = mockFile{}
	reply = rpc_common.ReadReply{}

	mFile.On("ReadAt", make([]byte, 10), int64(0)).Return(0, syscall.EACCES)
	dfFSOps.fds = map[uintptr]file{29: mFile}

	err = dfFSOps.Read(rpc_common.ReadRequest{FD: 29, Offset: 0, Num: 10}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EACCES"), err)
	}
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.ReadReply{}, reply)

	// *** Testing invalid FD
	mFS = mockFS{}
	mFile = mockFile{}
	reply = rpc_common.ReadReply{}
	mFile.On("ReadAt", make([]byte, 10), int64(0)).Return(10, nil)
	dfFSOps.fds = map[uintptr]file{30: mFile}

	err = dfFSOps.Read(rpc_common.ReadRequest{FD: 29, Offset: 0, Num: 10}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFile.AssertNotCalled(t, "ReadAt", mock.Anything, mock.Anything)
	assert.Equal(t, rpc_common.ReadReply{}, reply)

	// *** Testing happy path on ReadAt
	mFS = mockFS{}
	mFile = mockFile{}
	reply = rpc_common.ReadReply{}
	mFile.On("ReadAt", make([]byte, 5), int64(3)).Return(5, nil).Run(
		func(args mock.Arguments) {
			data := args.Get(0).([]byte)
			num := args.Get(1).(int64)
			for i := 0; i < len(data); i++ {
				data[i] = byte(i + int(num))
			}
		},
	)
	dfFSOps.fds = map[uintptr]file{29: mFile}

	err = dfFSOps.Read(rpc_common.ReadRequest{FD: 29, Offset: 3, Num: 5}, &reply)

	assert.NoError(t, err)
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.ReadReply{Data: []byte{3, 4, 5, 6, 7}}, reply)
}

func TestSeek(t *testing.T) {
	// *** Setup
	dfFSOps := NewDockerFuseFSOps()
	var (
		mFS   mockFS
		mFile mockFile
		reply rpc_common.SeekReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem

	// *** Testing error on Close
	mFile = mockFile{}
	reply = rpc_common.SeekReply{}
	mFile.On("Seek", int64(10), 0).Return(int64(0), syscall.EACCES)
	dfFSOps.fds = map[uintptr]file{29: mFile}

	err = dfFSOps.Seek(rpc_common.SeekRequest{FD: 29, Offset: 10, Whence: 0}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EACCES"), err)
	}
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.SeekReply{}, reply)

	// *** Testing invalid FD
	mFile = mockFile{}
	reply = rpc_common.SeekReply{}
	mFile.On("Seek", int64(0), 0).Return(int64(0), nil)
	dfFSOps.fds = map[uintptr]file{30: mFile}

	err = dfFSOps.Seek(rpc_common.SeekRequest{FD: 29, Offset: 0, Whence: 0}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFile.AssertNotCalled(t, "Seek", mock.Anything, mock.Anything)
	assert.Equal(t, rpc_common.SeekReply{}, reply)

	// *** Testing happy path for Seek
	mFile = mockFile{}
	reply = rpc_common.SeekReply{}
	mFile.On("Seek", int64(10), 0).Return(int64(10), nil)
	dfFSOps.fds = map[uintptr]file{29: mFile}

	err = dfFSOps.Seek(rpc_common.SeekRequest{FD: 29, Offset: 10, Whence: 0}, &reply)

	assert.NoError(t, err)
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.SeekReply{Num: 10}, reply)
}

func TestWrite(t *testing.T) {
	// *** Setup
	dfFSOps := NewDockerFuseFSOps()
	var (
		mFS   mockFS
		mFile mockFile
		reply rpc_common.WriteReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem

	// *** Testing error on WriteAt
	mFile = mockFile{}
	reply = rpc_common.WriteReply{}
	data := []byte{29, 30, 31, 21}
	mFile.On("WriteAt", data, int64(0)).Return(0, syscall.EACCES)
	dfFSOps.fds = map[uintptr]file{29: mFile}

	err = dfFSOps.Write(rpc_common.WriteRequest{FD: 29, Offset: 0, Data: data}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EACCES"), err)
	}
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.WriteReply{}, reply)

	// *** Testing invalid FD
	mFile = mockFile{}
	reply = rpc_common.WriteReply{}
	mFile.On("WriteAt", data, int64(0)).Return(10, nil)
	dfFSOps.fds = map[uintptr]file{30: mFile}

	err = dfFSOps.Write(rpc_common.WriteRequest{FD: 29, Offset: 0, Data: data}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFile.AssertNotCalled(t, "WriteAt", mock.Anything, mock.Anything)
	assert.Equal(t, rpc_common.WriteReply{}, reply)

	// *** Testing happy path on WriteAt
	mFile = mockFile{}
	reply = rpc_common.WriteReply{}
	mFile.On("WriteAt", data, int64(3)).Return(len(data), nil)
	dfFSOps.fds = map[uintptr]file{29: mFile}

	err = dfFSOps.Write(rpc_common.WriteRequest{FD: 29, Offset: 3, Data: data}, &reply)

	assert.NoError(t, err)
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.WriteReply{Num: len(data)}, reply)
}

func TestUnlink(t *testing.T) {
	// *** Setup
	var (
		mFS   mockFS
		reply rpc_common.UnlinkReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()

	// *** Testing error on Remove
	mFS = mockFS{}
	mFS.On("Remove", "/test/error_on_openfile").Return(syscall.ENOENT)

	reply = rpc_common.UnlinkReply{}
	err = dfFSOps.Unlink(rpc_common.UnlinkRequest{FullPath: "/test/error_on_openfile"}, &reply)

	assert.Equal(t, rpc_common.UnlinkReply{}, reply)
	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)

	// *** Testing happy path
	mFS = mockFS{}
	mFS.On("Remove", "/test/happy_path").Return(nil)

	reply = rpc_common.UnlinkReply{}
	err = dfFSOps.Unlink(rpc_common.UnlinkRequest{FullPath: "/test/happy_path"}, &reply)

	assert.Equal(t, rpc_common.UnlinkReply{}, reply)
	assert.NoError(t, err)
	mFS.AssertExpectations(t)
}

func TestFsync(t *testing.T) {
	// *** Setup
	dfFSOps := NewDockerFuseFSOps()
	var (
		mFS   mockFS
		mFile mockFile
		reply rpc_common.FsyncReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem

	// *** Testing error on Fsync
	mFile = mockFile{}
	reply = rpc_common.FsyncReply{}
	mFile.On("Sync").Return(syscall.EACCES)
	dfFSOps.fds = map[uintptr]file{29: mFile}

	err = dfFSOps.Fsync(rpc_common.FsyncRequest{FD: 29}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EACCES"), err)
	}
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.FsyncReply{}, reply)

	// *** Testing invalid FD
	mFile = mockFile{}
	reply = rpc_common.FsyncReply{}
	mFile.On("Sync").Return(nil)
	dfFSOps.fds = map[uintptr]file{30: mFile}

	err = dfFSOps.Fsync(rpc_common.FsyncRequest{FD: 29}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFile.AssertNotCalled(t, "Sync")
	assert.Equal(t, rpc_common.FsyncReply{}, reply)

	// *** Testing happy path
	mFile = mockFile{}
	reply = rpc_common.FsyncReply{}
	mFile.On("Sync").Return(nil)
	dfFSOps.fds = map[uintptr]file{29: mFile}

	err = dfFSOps.Fsync(rpc_common.FsyncRequest{FD: 29}, &reply)

	assert.NoError(t, err)
	mFile.AssertExpectations(t)
	assert.Equal(t, rpc_common.FsyncReply{}, reply)
}

func TestMkdir(t *testing.T) {
	// *** Setup
	var (
		mFS   mockFS
		mFI   mockFileInfo
		reply rpc_common.MkdirReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()

	// *** Testing error on Mkdir
	mFS = mockFS{}
	mFS.On("Mkdir", "/test/error_on_mkdir").Return(syscall.ENOENT)

	reply = rpc_common.MkdirReply{}
	err = dfFSOps.Mkdir(rpc_common.MkdirRequest{FullPath: "/test/error_on_mkdir"}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.MkdirReply{}, reply)

	// *** Testing error on Lstat
	mFS = mockFS{}
	mFI = mockFileInfo{}
	mFS.On("Mkdir", "/test/error_on_lstat").Return(nil)
	mFS.On("Lstat", "/test/error_on_lstat").Return(mFI, syscall.EINVAL)

	reply = rpc_common.MkdirReply{}
	err = dfFSOps.Mkdir(rpc_common.MkdirRequest{FullPath: "/test/error_on_lstat"}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.MkdirReply{}, reply)

	// *** Testing happy path
	mFS = mockFS{}
	mFI = mockFileInfo{}
	dfFSOps.fds = map[uintptr]file{}
	mFI.On("Sys").Return(&syscall.Stat_t{
		Mode:    0730,
		Nlink:   1,
		Ino:     29,
		Uid:     1,
		Gid:     2,
		Size:    1024,
		Blocks:  1,
		Blksize: 1024,
	})
	mFS.On("Mkdir", "/test/openfile_reg").Return(nil)
	mFS.On("Lstat", "/test/openfile_reg").Return(mFI, nil)
	mFS.On("Readlink", "/test/openfile_reg").Return("", nil)

	reply = rpc_common.MkdirReply{}
	err = dfFSOps.Mkdir(rpc_common.MkdirRequest{
		FullPath: "/test/openfile_reg",
		Mode:     fs.FileMode(0730),
	}, &reply)

	assert.NoError(t, err)
	mFI.AssertExpectations(t)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpc_common.MkdirReply{
		Mode:       0730,
		Nlink:      1,
		Ino:        29,
		Uid:        1,
		Gid:        2,
		Size:       1024,
		Blocks:     1,
		Blksize:    1024,
		LinkTarget: "",
	}, reply)
}
