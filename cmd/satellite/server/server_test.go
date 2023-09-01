package server

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/dguerri/dockerfuse/pkg/rpccommon"
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
	mdes := args.Get(0).([]*mockDirEntry)
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
func (o *mockFS) Rename(a, b string) error            { args := o.Called(a, b); return args.Error(0) }
func (o *mockFS) Link(a, b string) error              { args := o.Called(a, b); return args.Error(0) }
func (o *mockFS) Symlink(a, b string) error           { args := o.Called(a, b); return args.Error(0) }
func (o *mockFS) Chmod(n string, m os.FileMode) error { args := o.Called(n, m); return args.Error(0) }
func (o *mockFS) Chown(n string, u, g int) error      { args := o.Called(n, u, g); return args.Error(0) }
func (o *mockFS) Truncate(n string, s int64) error    { args := o.Called(n, s); return args.Error(0) }
func (o *mockFS) UtimesNano(p string, t []syscall.Timespec) error {
	args := o.Called(p, t)
	return args.Error(0)
}

// mockFileInfo implements mock os.FileInfo for testing
type mockFileInfo struct{ mock.Mock }

func (f *mockFileInfo) Size() int64        { a := f.Called(); return a.Get(0).(int64) }
func (f *mockFileInfo) Mode() os.FileMode  { a := f.Called(); return a.Get(0).(os.FileMode) }
func (f *mockFileInfo) ModTime() time.Time { a := f.Called(); return a.Get(0).(time.Time) }
func (f *mockFileInfo) Sys() any           { a := f.Called(); return a.Get(0).(*syscall.Stat_t) }
func (f *mockFileInfo) Name() string       { a := f.Called(); return a.String(0) }
func (f *mockFileInfo) IsDir() bool        { f.Called(); return f.Mode().IsDir() }

// mockDirEntry implements mock os.DirEntry for testing
type mockDirEntry struct{ mock.Mock }

func (d *mockDirEntry) Name() string      { a := d.Called(); return a.String(0) }
func (d *mockDirEntry) IsDir() bool       { d.Called(); return d.Type().IsDir() }
func (d *mockDirEntry) Type() fs.FileMode { a := d.Called(); return a.Get(0).(fs.FileMode) }
func (d *mockDirEntry) Info() (fs.FileInfo, error) {
	a := d.Called()
	return a.Get(0).(*mockFileInfo), a.Error(1)
}

// mockMockFile implements mock os.File for testing
type mockFile struct{ mock.Mock }

func (f *mockFile) Fd() uintptr                { a := f.Called(); return a.Get(0).(uintptr) }
func (f *mockFile) Close() error               { a := f.Called(); return a.Error(0) }
func (f *mockFile) Read(p []byte) (int, error) { a := f.Called(p); return a.Int(0), a.Error(0) }
func (f *mockFile) ReadAt(p []byte, o int64) (int, error) {
	a := f.Called(p, o)
	return a.Int(0), a.Error(1)
}
func (f *mockFile) Seek(o int64, w int) (int64, error) {
	a := f.Called(o, w)
	return a.Get(0).(int64), a.Error(1)
}
func (f *mockFile) Write(p []byte) (int, error) { a := f.Called(p); return a.Int(0), a.Error(0) }
func (f *mockFile) WriteAt(p []byte, o int64) (int, error) {
	a := f.Called(p, o)
	return a.Int(0), a.Error(1)
}
func (f *mockFile) Stat() (os.FileInfo, error) {
	a := f.Called()
	return a.Get(0).(*mockFileInfo), a.Error(1)
}
func (f *mockFile) Sync() error { a := f.Called(); return a.Error(0) }

func TestStat(t *testing.T) {
	// *** Setup
	var (
		mFS   mockFS
		mFI   mockFileInfo
		mFile mockFile
		reply rpccommon.StatReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()

	// *** Testing error on Lstat
	mFS = mockFS{}
	mFI = mockFileInfo{}
	reply = rpccommon.StatReply{}
	mFS.On("Lstat", "/test/error_on_lstat").Return(&mockFileInfo{}, syscall.ENOENT)

	err = dfFSOps.Stat(rpccommon.StatRequest{FullPath: "/test/error_on_lstat", UseFD: false}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)
	mFS.AssertNotCalled(t, "Readlink")
	assert.Equal(t, rpccommon.StatReply{}, reply)

	// *** Testing happy path on regular file
	mFS = mockFS{}
	dfFSOps.fds = map[uintptr]file{}
	mFI = mockFileInfo{}
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
	mFS.On("Lstat", "/test/reg").Return(&mFI, nil)
	mFS.On("Readlink", "/test/reg").Return("", nil)

	reply = rpccommon.StatReply{}
	err = dfFSOps.Stat(rpccommon.StatRequest{FullPath: "/test/reg", UseFD: false}, &reply)

	assert.NoError(t, err)
	mFI.AssertExpectations(t)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.StatReply{
		Mode:       0760,
		Nlink:      1,
		Ino:        29,
		UID:        1,
		GID:        2,
		Size:       29696,
		Blocks:     29,
		Blksize:    1024,
		LinkTarget: "",
	}, reply)

	// *** Testing happy path on regular file, error on Readlink
	mFS = mockFS{}
	dfFSOps.fds = map[uintptr]file{}
	mFI = mockFileInfo{}
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
	mFS.On("Lstat", "/test/reg").Return(&mFI, nil)
	mFS.On("Readlink", "/test/reg").Return("", syscall.EINVAL)

	reply = rpccommon.StatReply{}
	err = dfFSOps.Stat(rpccommon.StatRequest{FullPath: "/test/reg", UseFD: false}, &reply)

	assert.NoError(t, err)
	mFI.AssertExpectations(t)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.StatReply{
		Mode:       0760,
		Nlink:      1,
		Ino:        29,
		UID:        1,
		GID:        2,
		Size:       29696,
		Blocks:     29,
		Blksize:    1024,
		LinkTarget: "",
	}, reply)

	// *** Testing happy path on link
	mFS = mockFS{}
	dfFSOps.fds = map[uintptr]file{}
	mFI = mockFileInfo{}
	reply = rpccommon.StatReply{}
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
	mFS.On("Lstat", "/test/symlink").Return(&mFI, nil)
	mFS.On("Readlink", "/test/symlink").Return("/test/symlinktarget", nil)

	err = dfFSOps.Stat(rpccommon.StatRequest{FullPath: "/test/symlink", UseFD: false}, &reply)

	assert.NoError(t, err)
	mFI.AssertExpectations(t)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.StatReply{
		Mode:       0777,
		Nlink:      1,
		Ino:        29,
		UID:        1,
		GID:        2,
		Size:       4096,
		Blocks:     4,
		Blksize:    1024,
		LinkTarget: "/test/symlinktarget",
	}, reply)

	// *** Testing happy path on regular file, with FD
	mFS = mockFS{}
	dfFSOps.fds = map[uintptr]file{}
	mFI = mockFileInfo{}
	mFile = mockFile{}
	reply = rpccommon.StatReply{}
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
	mFile.On("Stat").Return(&mFI, nil)
	mFS.On("Lstat", "/test/reg").Return(&mFI, nil)
	mFS.On("Readlink", "/test/reg").Return("", nil)

	dfFSOps.fds[29] = &mFile
	err = dfFSOps.Stat(rpccommon.StatRequest{FullPath: "/test/reg", FD: 29, UseFD: true}, &reply)

	assert.NoError(t, err)
	mFI.AssertExpectations(t)
	mFile.AssertExpectations(t)
	mFS.AssertNotCalled(t, "Stat", mock.Anything)
	mFS.AssertNotCalled(t, "Readlink", mock.Anything)
	assert.Equal(t, rpccommon.StatReply{
		Mode:       0760,
		Nlink:      1,
		Ino:        29,
		UID:        1,
		GID:        2,
		Size:       29696,
		Blocks:     29,
		Blksize:    1024,
		LinkTarget: "",
	}, reply)

	// *** Testing error on retrieving FD
	mFS = mockFS{}
	dfFSOps.fds = map[uintptr]file{}
	mFI = mockFileInfo{}
	mFile = mockFile{}
	reply = rpccommon.StatReply{}
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
	mFile.On("Stat").Return(&mFI, nil)
	mFS.On("Lstat", "").Return(&mFI, nil)
	mFS.On("Readlink", "").Return("", nil)

	err = dfFSOps.Stat(rpccommon.StatRequest{FullPath: "", FD: 29, UseFD: true}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFI.AssertNotCalled(t, "Sys", mock.Anything)
	mFile.AssertNotCalled(t, "Stat", mock.Anything)
	mFS.AssertNotCalled(t, "Stat", mock.Anything)
	mFS.AssertNotCalled(t, "Readlink", mock.Anything)
	assert.Equal(t, rpccommon.StatReply{}, reply)
}

func TestReadDir(t *testing.T) {
	// *** Setup
	dfFSOps := NewDockerFuseFSOps()
	var (
		mFS   mockFS
		reply rpccommon.ReadDirReply
		err   error
		mFIs  []mockFileInfo
		mDIs  []*mockDirEntry
	)
	dfFS = &mFS // Set mock filesystem
	mFIs = []mockFileInfo{{}, {}, {}}
	mDIs = []*mockDirEntry{{}, {}, {}}

	// *** Testing error on ReadDir
	mFS = mockFS{}
	reply = rpccommon.ReadDirReply{}
	mFS.On("ReadDir", "/test/error_on_readdir").Return([]*mockDirEntry{}, syscall.ENOENT)

	err = dfFSOps.ReadDir(rpccommon.ReadDirRequest{FullPath: "/test/error_on_readdir"}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.ReadDirReply{}, reply)

	// *** Testing happy path
	mFS = mockFS{}
	reply = rpccommon.ReadDirReply{}
	mFIs[0].On("Sys").Return(&syscall.Stat_t{Ino: 29, Mode: 0660})
	mDIs[0].On("Name").Return("file1")
	mDIs[0].On("Info").Return(&mFIs[0], nil)
	mFIs[1].On("Sys").Return(&syscall.Stat_t{Ino: 30, Mode: 0777})
	mDIs[1].On("Name").Return("link1")
	mDIs[1].On("Info").Return(&mFIs[1], nil)
	mFIs[2].On("Sys").Return(&syscall.Stat_t{Ino: 31, Mode: 0755})
	mDIs[2].On("Name").Return("dir1")
	mDIs[2].On("Info").Return(&mFIs[2], nil)
	mFS.On("ReadDir", "/test/happy_path").Return(mDIs, nil)

	err = dfFSOps.ReadDir(rpccommon.ReadDirRequest{FullPath: "/test/happy_path"}, &reply)

	assert.NoError(t, err)
	for i := range mDIs {
		mDIs[i].AssertExpectations(t)
		mFIs[i].AssertExpectations(t)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.ReadDirReply{
		DirEntries: []rpccommon.DirEntry{
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
	reply = rpccommon.ReadDirReply{}
	mFS.On("ReadDir", "/test/happy_path").Return([]*mockDirEntry{}, nil)

	err = dfFSOps.ReadDir(rpccommon.ReadDirRequest{FullPath: "/test/happy_path"}, &reply)

	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.ReadDirReply{DirEntries: []rpccommon.DirEntry{}}, reply)
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
	reply = rpccommon.ReadDirReply{}
	mFIs[0].On("Sys").Return(&syscall.Stat_t{Ino: 29, Mode: 0660})
	mDIs[0].On("Name").Return("file1")
	mDIs[0].On("Info").Return(&mockFileInfo{}, fs.ErrNotExist)
	mFIs[1].On("Sys").Return(&syscall.Stat_t{Ino: 30, Mode: 0777})
	mDIs[1].On("Name").Return("link1")
	mDIs[1].On("Info").Return(&mFIs[1], nil)
	mFIs[2].On("Sys").Return(&syscall.Stat_t{Ino: 31, Mode: 0755})
	mDIs[2].On("Name").Return("dir1")
	mDIs[2].On("Info").Return(&mFIs[2], nil)
	mFS.On("ReadDir", "/test/info_err_no_exist").Return(mDIs, nil)

	err = dfFSOps.ReadDir(rpccommon.ReadDirRequest{FullPath: "/test/info_err_no_exist"}, &reply)

	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.ReadDirReply{
		DirEntries: []rpccommon.DirEntry{
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
	reply = rpccommon.ReadDirReply{}
	mFIs[0].On("Sys").Return(&syscall.Stat_t{Ino: 29, Mode: 0660})
	mDIs[0].On("Name").Return("file1")
	mDIs[0].On("Info").Return(&mockFileInfo{}, syscall.EINVAL)
	mFIs[1].On("Sys").Return(&syscall.Stat_t{Ino: 30, Mode: 0777})
	mDIs[1].On("Name").Return("link1")
	mDIs[1].On("Info").Return(&mFIs[1], nil)
	mFIs[2].On("Sys").Return(&syscall.Stat_t{Ino: 31, Mode: 0755})
	mDIs[2].On("Name").Return("dir1")
	mDIs[2].On("Info").Return(&mFIs[2], nil)
	mFS.On("ReadDir", "/test/info_err_unexpected").Return(mDIs, nil)

	err = dfFSOps.ReadDir(rpccommon.ReadDirRequest{FullPath: "/test/info_err_unexpected"}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EIO"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.ReadDirReply{DirEntries: []rpccommon.DirEntry{}}, reply)
}

func TestOpen(t *testing.T) {
	// *** Setup
	var (
		mFS   mockFS
		mFI   mockFileInfo
		mFile mockFile
		reply rpccommon.OpenReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()

	// *** Testing error on OpenFile
	mFS = mockFS{}
	dfFSOps.fds = map[uintptr]file{}
	mFS.On("OpenFile", "/test/error_on_openfile", syscall.O_CREAT|syscall.O_RDWR, fs.FileMode(0666)).Return(&mockFile{}, syscall.ENOENT)

	reply = rpccommon.OpenReply{}
	err = dfFSOps.Open(rpccommon.OpenRequest{
		FullPath: "/test/error_on_openfile",
		SAFlags:  rpccommon.SystemToSAFlags(syscall.O_CREAT | syscall.O_RDWR),
		Mode:     fs.FileMode(0666),
	}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.OpenReply{}, reply)

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
	mFile.On("Stat").Return(&mFI, nil)
	mFS.On("OpenFile", "/test/openfile_reg", syscall.O_RDWR, fs.FileMode(0640)).Return(&mFile, nil)
	mFS.On("Readlink", "/test/openfile_reg").Return("", nil)

	reply = rpccommon.OpenReply{}
	err = dfFSOps.Open(rpccommon.OpenRequest{
		FullPath: "/test/openfile_reg",
		SAFlags:  rpccommon.SystemToSAFlags(syscall.O_RDWR),
		Mode:     fs.FileMode(0640),
	}, &reply)

	assert.NoError(t, err)
	mFI.AssertExpectations(t)
	mFile.AssertExpectations(t)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.OpenReply{
		FD: 29,
		StatReply: rpccommon.StatReply{
			Mode:       0660,
			Nlink:      2,
			Ino:        29,
			UID:        1,
			GID:        2,
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
	mFile.On("Stat").Return(&mFI, nil)
	mFS.On("OpenFile", "/test/openfile_symlink", syscall.O_RDWR, fs.FileMode(0640)).Return(&mFile, nil)
	mFS.On("Readlink", "/test/openfile_symlink").Return("/test/openfile_symlink_target", nil)
	dfFSOps.fds = map[uintptr]file{29: &mFile}

	reply = rpccommon.OpenReply{}
	err = dfFSOps.Open(rpccommon.OpenRequest{
		FullPath: "/test/openfile_symlink",
		SAFlags:  rpccommon.SystemToSAFlags(syscall.O_RDWR),
		Mode:     fs.FileMode(0640),
	}, &reply)

	assert.NoError(t, err)
	mFI.AssertExpectations(t)
	mFile.AssertExpectations(t)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.OpenReply{
		FD: 29,
		StatReply: rpccommon.StatReply{
			Mode:       0777,
			Nlink:      1,
			Ino:        29,
			UID:        1,
			GID:        2,
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
	mFile.On("Stat").Return(&mFI, nil)
	mFS.On("OpenFile", "/test/openfile_reg", syscall.O_RDWR, fs.FileMode(0640)).Return(&mFile, nil)
	mFS.On("Readlink", "/test/openfile_reg").Return("", syscall.EINVAL)

	reply = rpccommon.OpenReply{}
	err = dfFSOps.Open(rpccommon.OpenRequest{
		FullPath: "/test/openfile_reg",
		SAFlags:  rpccommon.SystemToSAFlags(syscall.O_RDWR),
		Mode:     fs.FileMode(0640),
	}, &reply)

	assert.NoError(t, err)
	mFI.AssertExpectations(t)
	mFile.AssertExpectations(t)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.OpenReply{
		FD: 30,
		StatReply: rpccommon.StatReply{
			Mode:       0660,
			Nlink:      2,
			Ino:        29,
			UID:        1,
			GID:        2,
			Size:       3072,
			Blocks:     3,
			Blksize:    1024,
			LinkTarget: "",
		},
	}, reply)

	// *** Testing Open on a regular existing file error on fd.Stat()
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
	mFile.On("Stat").Return(&mFI, syscall.EINVAL)
	mFS.On("OpenFile", "/test/openfile_reg", syscall.O_RDWR, fs.FileMode(0640)).Return(&mFile, nil)
	mFS.On("Readlink", "/test/openfile_reg").Return("", nil)

	reply = rpccommon.OpenReply{}
	err = dfFSOps.Open(rpccommon.OpenRequest{
		FullPath: "/test/openfile_reg",
		SAFlags:  rpccommon.SystemToSAFlags(syscall.O_RDWR),
		Mode:     fs.FileMode(0640),
	}, &reply)

	assert.Equal(t, rpccommon.OpenReply{}, reply)
	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFile.AssertExpectations(t)
	mFS.AssertCalled(t, "OpenFile", "/test/openfile_reg", syscall.O_RDWR, fs.FileMode(0640))
	mFS.AssertNotCalled(t, "ReadLink", mock.Anything)
	mFI.AssertNotCalled(t, "Sys", mock.Anything)
}

func TestClose(t *testing.T) {
	// *** Setup
	dfFSOps := NewDockerFuseFSOps()
	var (
		mFS   mockFS
		mFile mockFile
		reply rpccommon.CloseReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem

	// *** Testing error on Close
	mFS = mockFS{}
	mFile = mockFile{}
	reply = rpccommon.CloseReply{}
	mFile.On("Close").Return(syscall.EACCES)
	dfFSOps.fds = map[uintptr]file{29: &mFile}

	err = dfFSOps.Close(rpccommon.CloseRequest{FD: 29}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EACCES"), err)
	}
	mFile.AssertExpectations(t)
	assert.Equal(t, rpccommon.CloseReply{}, reply)

	// *** Testing invalid FD
	mFS = mockFS{}
	mFile = mockFile{}
	reply = rpccommon.CloseReply{}
	mFile.On("Close").Return(nil)
	dfFSOps.fds = map[uintptr]file{30: &mFile}

	err = dfFSOps.Close(rpccommon.CloseRequest{FD: 29}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFile.AssertNotCalled(t, "Close")
	assert.Equal(t, rpccommon.CloseReply{}, reply)

	// *** Testing happy path
	mFS = mockFS{}
	mFile = mockFile{}
	reply = rpccommon.CloseReply{}
	mFile.On("Close").Return(nil)
	dfFSOps.fds = map[uintptr]file{29: &mFile}

	err = dfFSOps.Close(rpccommon.CloseRequest{FD: 29}, &reply)

	assert.NoError(t, err)
	mFile.AssertExpectations(t)
	assert.Equal(t, rpccommon.CloseReply{}, reply)
}

func TestRead(t *testing.T) {
	// *** Setup
	dfFSOps := NewDockerFuseFSOps()
	var (
		mFS    mockFS
		mFile  mockFile
		reply  rpccommon.ReadReply
		err    error
		offset int64
	)
	dfFS = &mFS // Set mock filesystem

	// *** Testing error on ReadAt
	mFS = mockFS{}
	mFile = mockFile{}
	reply = rpccommon.ReadReply{}

	mFile.On("ReadAt", make([]byte, 10), int64(0)).Return(0, syscall.EACCES)
	dfFSOps.fds = map[uintptr]file{29: &mFile}

	err = dfFSOps.Read(rpccommon.ReadRequest{FD: 29, Offset: 0, Num: 10}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EACCES"), err)
	}
	mFile.AssertExpectations(t)
	assert.Equal(t, rpccommon.ReadReply{}, reply)

	// *** Testing invalid FD
	mFS = mockFS{}
	mFile = mockFile{}
	reply = rpccommon.ReadReply{}
	mFile.On("ReadAt", make([]byte, 10), int64(0)).Return(10, nil)
	dfFSOps.fds = map[uintptr]file{30: &mFile}

	err = dfFSOps.Read(rpccommon.ReadRequest{FD: 29, Offset: 0, Num: 10}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFile.AssertNotCalled(t, "ReadAt", mock.Anything, mock.Anything)
	assert.Equal(t, rpccommon.ReadReply{}, reply)

	// *** Testing happy path on ReadAt
	mFS = mockFS{}
	mFile = mockFile{}
	reply = rpccommon.ReadReply{}
	mFile.On("ReadAt", make([]byte, 5), int64(3)).Return(5, nil).Run(
		func(args mock.Arguments) {
			data := args.Get(0).([]byte)
			num := args.Get(1).(int64)
			for i := 0; i < len(data); i++ {
				data[i] = byte(i + int(num))
			}
		},
	)
	dfFSOps.fds = map[uintptr]file{29: &mFile}

	err = dfFSOps.Read(rpccommon.ReadRequest{FD: 29, Offset: 3, Num: 5}, &reply)

	assert.NoError(t, err)
	mFile.AssertExpectations(t)
	assert.Equal(t, rpccommon.ReadReply{Data: []byte{3, 4, 5, 6, 7}}, reply)

	// *** Testing small file on ReadAt
	// size of the buffer > data we actually have in file
	mFS = mockFS{}
	mFile = mockFile{}
	reply = rpccommon.ReadReply{}
	offset = 0
	mFile.On("ReadAt", make([]byte, 32), int64(offset)).Return(5, io.EOF).Run(
		func(args mock.Arguments) {
			data := args.Get(0).([]byte)
			// num := args.Get(1).(int64)
			for i := 0; i < 5; i++ { // our file is 5 bytes long
				data[i] = byte(i + 1)
			}
		},
	)
	dfFSOps.fds = map[uintptr]file{29: &mFile}

	err = dfFSOps.Read(rpccommon.ReadRequest{FD: 29, Offset: offset, Num: 32}, &reply)

	assert.NoError(t, err)
	mFile.AssertExpectations(t)
	assert.Equal(t, rpccommon.ReadReply{Data: []byte{1, 2, 3, 4, 5}}, reply)
}

func TestSeek(t *testing.T) {
	// *** Setup
	dfFSOps := NewDockerFuseFSOps()
	var (
		mFS   mockFS
		mFile mockFile
		reply rpccommon.SeekReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem

	// *** Testing error on Close
	mFile = mockFile{}
	reply = rpccommon.SeekReply{}
	mFile.On("Seek", int64(10), 0).Return(int64(0), syscall.EACCES)
	dfFSOps.fds = map[uintptr]file{29: &mFile}

	err = dfFSOps.Seek(rpccommon.SeekRequest{FD: 29, Offset: 10, Whence: 0}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EACCES"), err)
	}
	mFile.AssertExpectations(t)
	assert.Equal(t, rpccommon.SeekReply{}, reply)

	// *** Testing invalid FD
	mFile = mockFile{}
	reply = rpccommon.SeekReply{}
	mFile.On("Seek", int64(0), 0).Return(int64(0), nil)
	dfFSOps.fds = map[uintptr]file{30: &mFile}

	err = dfFSOps.Seek(rpccommon.SeekRequest{FD: 29, Offset: 0, Whence: 0}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFile.AssertNotCalled(t, "Seek", mock.Anything, mock.Anything)
	assert.Equal(t, rpccommon.SeekReply{}, reply)

	// *** Testing happy path for Seek
	mFile = mockFile{}
	reply = rpccommon.SeekReply{}
	mFile.On("Seek", int64(10), 0).Return(int64(10), nil)
	dfFSOps.fds = map[uintptr]file{29: &mFile}

	err = dfFSOps.Seek(rpccommon.SeekRequest{FD: 29, Offset: 10, Whence: 0}, &reply)

	assert.NoError(t, err)
	mFile.AssertExpectations(t)
	assert.Equal(t, rpccommon.SeekReply{Num: 10}, reply)
}

func TestWrite(t *testing.T) {
	// *** Setup
	dfFSOps := NewDockerFuseFSOps()
	var (
		mFS   mockFS
		mFile mockFile
		reply rpccommon.WriteReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem

	// *** Testing error on WriteAt
	mFile = mockFile{}
	reply = rpccommon.WriteReply{}
	data := []byte{29, 30, 31, 21}
	mFile.On("WriteAt", data, int64(0)).Return(0, syscall.EACCES)
	dfFSOps.fds = map[uintptr]file{29: &mFile}

	err = dfFSOps.Write(rpccommon.WriteRequest{FD: 29, Offset: 0, Data: data}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EACCES"), err)
	}
	mFile.AssertExpectations(t)
	assert.Equal(t, rpccommon.WriteReply{}, reply)

	// *** Testing invalid FD
	mFile = mockFile{}
	reply = rpccommon.WriteReply{}
	mFile.On("WriteAt", data, int64(0)).Return(10, nil)
	dfFSOps.fds = map[uintptr]file{30: &mFile}

	err = dfFSOps.Write(rpccommon.WriteRequest{FD: 29, Offset: 0, Data: data}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFile.AssertNotCalled(t, "WriteAt", mock.Anything, mock.Anything)
	assert.Equal(t, rpccommon.WriteReply{}, reply)

	// *** Testing happy path on WriteAt
	mFile = mockFile{}
	reply = rpccommon.WriteReply{}
	mFile.On("WriteAt", data, int64(3)).Return(len(data), nil)
	dfFSOps.fds = map[uintptr]file{29: &mFile}

	err = dfFSOps.Write(rpccommon.WriteRequest{FD: 29, Offset: 3, Data: data}, &reply)

	assert.NoError(t, err)
	mFile.AssertExpectations(t)
	assert.Equal(t, rpccommon.WriteReply{Num: len(data)}, reply)
}

func TestUnlink(t *testing.T) {
	// *** Setup
	var (
		mFS   mockFS
		reply rpccommon.UnlinkReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()

	// *** Testing error on Remove
	mFS = mockFS{}
	mFS.On("Remove", "/test/error_on_openfile").Return(syscall.ENOENT)

	reply = rpccommon.UnlinkReply{}
	err = dfFSOps.Unlink(rpccommon.UnlinkRequest{FullPath: "/test/error_on_openfile"}, &reply)

	assert.Equal(t, rpccommon.UnlinkReply{}, reply)
	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)

	// *** Testing happy path
	mFS = mockFS{}
	mFS.On("Remove", "/test/happy_path").Return(nil)

	reply = rpccommon.UnlinkReply{}
	err = dfFSOps.Unlink(rpccommon.UnlinkRequest{FullPath: "/test/happy_path"}, &reply)

	assert.Equal(t, rpccommon.UnlinkReply{}, reply)
	assert.NoError(t, err)
	mFS.AssertExpectations(t)
}

func TestFsync(t *testing.T) {
	// *** Setup
	dfFSOps := NewDockerFuseFSOps()
	var (
		mFS   mockFS
		mFile mockFile
		reply rpccommon.FsyncReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem

	// *** Testing error on Fsync
	mFile = mockFile{}
	reply = rpccommon.FsyncReply{}
	mFile.On("Sync").Return(syscall.EACCES)
	dfFSOps.fds = map[uintptr]file{29: &mFile}

	err = dfFSOps.Fsync(rpccommon.FsyncRequest{FD: 29}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EACCES"), err)
	}
	mFile.AssertExpectations(t)
	assert.Equal(t, rpccommon.FsyncReply{}, reply)

	// *** Testing invalid FD
	mFile = mockFile{}
	reply = rpccommon.FsyncReply{}
	mFile.On("Sync").Return(nil)
	dfFSOps.fds = map[uintptr]file{30: &mFile}

	err = dfFSOps.Fsync(rpccommon.FsyncRequest{FD: 29}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFile.AssertNotCalled(t, "Sync")
	assert.Equal(t, rpccommon.FsyncReply{}, reply)

	// *** Testing happy path
	mFile = mockFile{}
	reply = rpccommon.FsyncReply{}
	mFile.On("Sync").Return(nil)
	dfFSOps.fds = map[uintptr]file{29: &mFile}

	err = dfFSOps.Fsync(rpccommon.FsyncRequest{FD: 29}, &reply)

	assert.NoError(t, err)
	mFile.AssertExpectations(t)
	assert.Equal(t, rpccommon.FsyncReply{}, reply)
}

func TestMkdir(t *testing.T) {
	// *** Setup
	var (
		mFS   mockFS
		mFI   mockFileInfo
		reply rpccommon.MkdirReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()

	// *** Testing error on Mkdir
	mFS = mockFS{}
	mFS.On("Mkdir", "/test/error_on_mkdir").Return(syscall.ENOENT)

	reply = rpccommon.MkdirReply{}
	err = dfFSOps.Mkdir(rpccommon.MkdirRequest{FullPath: "/test/error_on_mkdir"}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.MkdirReply{}, reply)

	// *** Testing error on Lstat
	mFS = mockFS{}
	mFI = mockFileInfo{}
	mFS.On("Mkdir", "/test/error_on_lstat").Return(nil)
	mFS.On("Lstat", "/test/error_on_lstat").Return(&mFI, syscall.EINVAL)

	reply = rpccommon.MkdirReply{}
	err = dfFSOps.Mkdir(rpccommon.MkdirRequest{FullPath: "/test/error_on_lstat"}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.MkdirReply{}, reply)

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
	mFS.On("Lstat", "/test/openfile_reg").Return(&mFI, nil)
	mFS.On("Readlink", "/test/openfile_reg").Return("", nil)

	reply = rpccommon.MkdirReply{}
	err = dfFSOps.Mkdir(rpccommon.MkdirRequest{
		FullPath: "/test/openfile_reg",
		Mode:     fs.FileMode(0730),
	}, &reply)

	assert.NoError(t, err)
	mFI.AssertExpectations(t)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.MkdirReply{
		Mode:       0730,
		Nlink:      1,
		Ino:        29,
		UID:        1,
		GID:        2,
		Size:       1024,
		Blocks:     1,
		Blksize:    1024,
		LinkTarget: "",
	}, reply)
}

func TestRmdir(t *testing.T) {
	// *** Setup
	var (
		mFS   mockFS
		reply rpccommon.RmdirReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()

	// *** Testing directory not empty error on Remove
	mFS = mockFS{}
	mFS.On("Remove", "/test/error_on_remove").Return(syscall.ENOTEMPTY)

	reply = rpccommon.RmdirReply{}
	err = dfFSOps.Rmdir(rpccommon.RmdirRequest{FullPath: "/test/error_on_remove"}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOTEMPTY"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.RmdirReply{}, reply)

	// *** Testing happy path
	mFS = mockFS{}
	mFS.On("Remove", "/test/remove_dir").Return(nil)

	reply = rpccommon.RmdirReply{}
	err = dfFSOps.Rmdir(rpccommon.RmdirRequest{FullPath: "/test/remove_dir"}, &reply)

	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.RmdirReply{}, reply)
}

func TestRename(t *testing.T) {
	// *** Setup
	var (
		mFS   mockFS
		reply rpccommon.RenameReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()

	// *** Testing error on Rename
	mFS = mockFS{}
	mFS.On("Rename", "/test/error_on_rename", "/test/error_on_rename_new").Return(syscall.ENOENT)

	reply = rpccommon.RenameReply{}
	err = dfFSOps.Rename(rpccommon.RenameRequest{
		FullPath:    "/test/error_on_rename",
		FullNewPath: "/test/error_on_rename_new",
	}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.RenameReply{}, reply)

	// *** Testing happy path
	mFS = mockFS{}
	mFS.On("Rename", "/test/a_file", "/test/a_new_file").Return(nil)

	reply = rpccommon.RenameReply{}
	err = dfFSOps.Rename(rpccommon.RenameRequest{
		FullPath:    "/test/a_file",
		FullNewPath: "/test/a_new_file",
	}, &reply)

	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.RenameReply{}, reply)
}

func TestReadlink(t *testing.T) {
	// *** Setup
	var (
		mFS   mockFS
		reply rpccommon.ReadlinkReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()

	// *** Testing error on Readlink
	mFS = mockFS{}
	mFS.On("Readlink", "/test/error_on_readlink").Return("", syscall.ENOENT)

	reply = rpccommon.ReadlinkReply{}
	err = dfFSOps.Readlink(rpccommon.ReadlinkRequest{FullPath: "/test/error_on_readlink"}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.ReadlinkReply{}, reply)

	// *** Testing happy path
	mFS = mockFS{}
	mFS.On("Readlink", "/test/a_file").Return("/test/a_file_target", nil)

	reply = rpccommon.ReadlinkReply{}
	err = dfFSOps.Readlink(rpccommon.ReadlinkRequest{FullPath: "/test/a_file"}, &reply)

	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.ReadlinkReply{LinkTarget: "/test/a_file_target"}, reply)
}

func TestLink(t *testing.T) {
	// *** Setup
	var (
		mFS   mockFS
		reply rpccommon.LinkReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()

	// *** Testing error on Link
	mFS = mockFS{}
	mFS.On("Link", "/test/error_on_link", "/test/error_on_link_bis").Return(syscall.ENOENT)

	reply = rpccommon.LinkReply{}
	err = dfFSOps.Link(rpccommon.LinkRequest{
		OldFullPath: "/test/error_on_link",
		NewFullPath: "/test/error_on_link_bis",
	}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.LinkReply{}, reply)

	// *** Testing happy path
	mFS = mockFS{}
	mFS.On("Link", "/test/a_file", "/test/another_file").Return(nil)

	reply = rpccommon.LinkReply{}
	err = dfFSOps.Link(rpccommon.LinkRequest{
		OldFullPath: "/test/a_file",
		NewFullPath: "/test/another_file",
	}, &reply)

	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.LinkReply{}, reply)
}

func TestSymlink(t *testing.T) {
	// *** Setup
	var (
		mFS   mockFS
		reply rpccommon.SymlinkReply
		err   error
	)
	dfFS = &mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()

	// *** Testing error on Symlink
	mFS = mockFS{}
	mFS.On("Symlink", "/test/error_on_symlink", "/test/error_on_symlink_bis").Return(syscall.ENOENT)

	reply = rpccommon.SymlinkReply{}
	err = dfFSOps.Symlink(rpccommon.SymlinkRequest{
		OldFullPath: "/test/error_on_symlink",
		NewFullPath: "/test/error_on_symlink_bis",
	}, &reply)

	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.SymlinkReply{}, reply)

	// *** Testing happy path
	mFS = mockFS{}
	mFS.On("Symlink", "/test/a_file", "/test/another_file").Return(nil)

	reply = rpccommon.SymlinkReply{}
	err = dfFSOps.Symlink(rpccommon.SymlinkRequest{
		OldFullPath: "/test/a_file",
		NewFullPath: "/test/another_file",
	}, &reply)

	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	assert.Equal(t, rpccommon.SymlinkReply{}, reply)
}

func TestSetAttr(t *testing.T) {
	// *** Setup
	var (
		mFS     mockFS
		mFI     mockFileInfo
		reply   rpccommon.SetAttrReply
		request rpccommon.SetAttrRequest
		err     error
	)
	dfFS = &mFS // Set mock filesystem
	dfFSOps := NewDockerFuseFSOps()

	// *** Testing error on Chmod
	mFS = mockFS{}
	mFI = mockFileInfo{}
	request = rpccommon.SetAttrRequest{FullPath: "/test/error_on_chmod"}
	request.SetMode(0666)
	request.SetUID(0)
	request.SetGID(1)
	request.SetATime(time.UnixMicro(1661073465))
	request.SetMTime(time.UnixMicro(1661073466))
	request.SetSize(29)
	mFS.On("Chmod", "/test/error_on_chmod", os.FileMode(0666)).Return(syscall.ENOENT)
	mFS.On("Chown", "/test/error_on_chmod", 0, 1).Return(nil)
	mFS.On("UtimesNano", "/test/error_on_chmod", []syscall.Timespec{
		syscall.NsecToTimespec(time.UnixMicro(1661073465).UnixNano()),
		syscall.NsecToTimespec(time.UnixMicro(1661073466).UnixNano()),
	}).Return(nil)
	mFS.On("Truncate", "/test/error_on_chmod", int64(29)).Return(nil)
	mFS.On("Lstat", "/test/error_on_chmod").Return(&mockFileInfo{}, nil)
	mFS.On("Readlink", "/test/error_on_chmod").Return("", nil)

	reply = rpccommon.SetAttrReply{}
	err = dfFSOps.SetAttr(request, &reply)

	assert.Equal(t, rpccommon.SetAttrReply{}, reply)
	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: ENOENT"), err)
	}

	mFS.AssertCalled(t, "Chmod", "/test/error_on_chmod", os.FileMode(0666))
	mFS.AssertNotCalled(t, "Chown", mock.Anything)
	mFS.AssertNotCalled(t, "UtimesNano", mock.Anything)
	mFS.AssertNotCalled(t, "Truncate", mock.Anything)
	mFS.AssertNotCalled(t, "Lstat", mock.Anything)
	mFS.AssertNotCalled(t, "Readlink", mock.Anything)

	// *** Testing error on Chown
	mFS = mockFS{}
	mFI = mockFileInfo{}
	request = rpccommon.SetAttrRequest{FullPath: "/test/error_on_chown"}
	request.SetMode(0666)
	request.SetUID(0)
	request.SetGID(1)
	request.SetATime(time.UnixMicro(1661073465))
	request.SetMTime(time.UnixMicro(1661073466))
	request.SetSize(29)
	mFS.On("Chmod", "/test/error_on_chown", os.FileMode(0666)).Return(nil)
	mFS.On("Chown", "/test/error_on_chown", 0, 1).Return(syscall.EACCES)
	mFS.On("UtimesNano", "/test/error_on_chown", []syscall.Timespec{
		syscall.NsecToTimespec(time.UnixMicro(1661073465).UnixNano()),
		syscall.NsecToTimespec(time.UnixMicro(1661073466).UnixNano()),
	}).Return(nil)
	mFS.On("Truncate", "/test/error_on_chown", int64(29)).Return(nil)
	mFS.On("Lstat", "/test/error_on_chmod").Return(&mockFileInfo{}, nil)
	mFS.On("Readlink", "/test/error_on_chmod").Return("", nil)

	reply = rpccommon.SetAttrReply{}
	err = dfFSOps.SetAttr(request, &reply)

	assert.Equal(t, rpccommon.SetAttrReply{}, reply)
	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EACCES"), err)
	}

	mFS.AssertCalled(t, "Chmod", "/test/error_on_chown", os.FileMode(0666))
	mFS.AssertCalled(t, "Chown", "/test/error_on_chown", 0, 1)
	mFS.AssertNotCalled(t, "UtimesNano", mock.Anything)
	mFS.AssertNotCalled(t, "Truncate", mock.Anything)
	mFS.AssertNotCalled(t, "Lstat", mock.Anything)
	mFS.AssertNotCalled(t, "Readlink", mock.Anything)

	// *** Testing error on UtimesNano
	mFS = mockFS{}
	mFI = mockFileInfo{}
	request = rpccommon.SetAttrRequest{FullPath: "/test/error_on_utimesnano"}
	request.SetMode(0666)
	request.SetUID(0)
	request.SetGID(1)
	request.SetATime(time.UnixMicro(1661073465))
	request.SetMTime(time.UnixMicro(1661073466))
	request.SetSize(29)
	mFS.On("Chmod", "/test/error_on_utimesnano", os.FileMode(0666)).Return(nil)
	mFS.On("Chown", "/test/error_on_utimesnano", 0, 1).Return(nil)
	mFS.On("UtimesNano", "/test/error_on_utimesnano", []syscall.Timespec{
		syscall.NsecToTimespec(time.UnixMicro(1661073465).UnixNano()),
		syscall.NsecToTimespec(time.UnixMicro(1661073466).UnixNano()),
	}).Return(syscall.EINVAL)
	mFS.On("Truncate", "/test/error_on_utimesnano", int64(29)).Return(nil)
	mFS.On("Lstat", "/test/error_on_chmod").Return(&mockFileInfo{}, nil)
	mFS.On("Readlink", "/test/error_on_chmod").Return("", nil)

	reply = rpccommon.SetAttrReply{}
	err = dfFSOps.SetAttr(request, &reply)

	assert.Equal(t, rpccommon.SetAttrReply{}, reply)
	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}

	mFS.AssertCalled(t, "Chmod", "/test/error_on_utimesnano", os.FileMode(0666))
	mFS.AssertCalled(t, "Chown", "/test/error_on_utimesnano", 0, 1)
	mFS.AssertCalled(t, "UtimesNano", "/test/error_on_utimesnano", []syscall.Timespec{
		syscall.NsecToTimespec(time.UnixMicro(1661073465).UnixNano()),
		syscall.NsecToTimespec(time.UnixMicro(1661073466).UnixNano()),
	})
	mFS.AssertNotCalled(t, "Truncate", mock.Anything)
	mFS.AssertNotCalled(t, "Lstat", mock.Anything)
	mFS.AssertNotCalled(t, "Readlink", mock.Anything)

	// *** Testing error on Truncate
	mFS = mockFS{}
	mFI = mockFileInfo{}
	request = rpccommon.SetAttrRequest{FullPath: "/test/error_on_truncate"}
	request.SetMode(0666)
	request.SetUID(0)
	request.SetGID(1)
	request.SetATime(time.UnixMicro(1661073465))
	request.SetMTime(time.UnixMicro(1661073466))
	request.SetSize(29)
	mFS.On("Chmod", "/test/error_on_truncate", os.FileMode(0666)).Return(nil)
	mFS.On("Chown", "/test/error_on_truncate", 0, 1).Return(nil)
	mFS.On("UtimesNano", "/test/error_on_truncate", []syscall.Timespec{
		syscall.NsecToTimespec(time.UnixMicro(1661073465).UnixNano()),
		syscall.NsecToTimespec(time.UnixMicro(1661073466).UnixNano()),
	}).Return(nil)
	mFS.On("Truncate", "/test/error_on_truncate", int64(29)).Return(syscall.EFAULT)
	mFS.On("Lstat", "/test/error_on_chmod").Return(&mockFileInfo{}, nil)
	mFS.On("Readlink", "/test/error_on_chmod").Return("", nil)

	reply = rpccommon.SetAttrReply{}
	err = dfFSOps.SetAttr(request, &reply)

	assert.Equal(t, rpccommon.SetAttrReply{}, reply)
	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EFAULT"), err)
	}

	mFS.AssertCalled(t, "Chmod", "/test/error_on_truncate", os.FileMode(0666))
	mFS.AssertCalled(t, "Chown", "/test/error_on_truncate", 0, 1)
	mFS.AssertCalled(t, "UtimesNano", "/test/error_on_truncate", []syscall.Timespec{
		syscall.NsecToTimespec(time.UnixMicro(1661073465).UnixNano()),
		syscall.NsecToTimespec(time.UnixMicro(1661073466).UnixNano()),
	})
	mFS.AssertCalled(t, "Truncate", "/test/error_on_truncate", int64(29))
	mFS.AssertNotCalled(t, "Lstat", mock.Anything)
	mFS.AssertNotCalled(t, "Readlink", mock.Anything)

	// *** Testing happy path, changing everything
	mFS = mockFS{}
	mFI = mockFileInfo{}
	request = rpccommon.SetAttrRequest{FullPath: "/test/happy_path"}
	request.SetMode(0666)
	request.SetUID(0)
	request.SetGID(1)
	request.SetATime(time.UnixMicro(1661073465))
	request.SetMTime(time.UnixMicro(1661073466))
	request.SetSize(29696)
	mFS.On("Chmod", "/test/happy_path", os.FileMode(0666)).Return(nil)
	mFS.On("Chown", "/test/happy_path", 0, 1).Return(nil)
	mFS.On("UtimesNano", "/test/happy_path", []syscall.Timespec{
		syscall.NsecToTimespec(time.UnixMicro(1661073465).UnixNano()),
		syscall.NsecToTimespec(time.UnixMicro(1661073466).UnixNano()),
	}).Return(nil)
	mFS.On("Truncate", "/test/happy_path", int64(29696)).Return(nil)
	mFI.On("Sys").Return(&syscall.Stat_t{
		Mode:    0666,
		Nlink:   1,
		Ino:     2929,
		Uid:     0,
		Gid:     1,
		Blocks:  29,
		Blksize: 1024,
		Size:    29696,
	})
	mFS.On("Lstat", "/test/happy_path").Return(&mFI, nil)
	mFS.On("Readlink", "/test/happy_path").Return("", nil)

	reply = rpccommon.SetAttrReply{}
	err = dfFSOps.SetAttr(request, &reply)

	assert.Equal(t, rpccommon.SetAttrReply{
		Mode:       0666,
		Nlink:      1,
		Ino:        2929,
		UID:        0,
		GID:        1,
		Size:       29696,
		Blocks:     29,
		Blksize:    1024,
		LinkTarget: "",
	}, reply)
	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	mFI.AssertExpectations(t)

	// *** Testing happy path, changing everything but atime
	mFS = mockFS{}
	mFI = mockFileInfo{}
	request = rpccommon.SetAttrRequest{FullPath: "/test/happy_path"}
	request.SetMode(0666)
	request.SetUID(0)
	request.SetGID(1)
	request.SetMTime(time.UnixMicro(1661073466))
	request.SetSize(29696)
	mFS.On("Chmod", "/test/happy_path", os.FileMode(0666)).Return(nil)
	mFS.On("Chown", "/test/happy_path", 0, 1).Return(nil)
	mFS.On("UtimesNano", "/test/happy_path", []syscall.Timespec{
		{Nsec: rpccommon.UTIME_OMIT},
		syscall.NsecToTimespec(time.UnixMicro(1661073466).UnixNano()),
	}).Return(nil)
	mFS.On("Truncate", "/test/happy_path", int64(29696)).Return(nil)
	mFI.On("Sys").Return(&syscall.Stat_t{
		Mode:    0666,
		Nlink:   1,
		Ino:     2929,
		Uid:     0,
		Gid:     1,
		Blocks:  29,
		Blksize: 1024,
		Size:    29696,
	})
	mFS.On("Lstat", "/test/happy_path").Return(&mFI, nil)
	mFS.On("Readlink", "/test/happy_path").Return("", nil)

	reply = rpccommon.SetAttrReply{}
	err = dfFSOps.SetAttr(request, &reply)

	assert.Equal(t, rpccommon.SetAttrReply{
		Mode:       0666,
		Nlink:      1,
		Ino:        2929,
		UID:        0,
		GID:        1,
		Size:       29696,
		Blocks:     29,
		Blksize:    1024,
		LinkTarget: "",
	}, reply)
	assert.NoError(t, err)
	mFS.AssertExpectations(t)
	mFI.AssertExpectations(t)

	// *** Testing happy path, changing everything but mtime and mode
	mFS = mockFS{}
	mFI = mockFileInfo{}
	request = rpccommon.SetAttrRequest{FullPath: "/test/happy_path"}
	request.SetUID(0)
	request.SetGID(1)
	request.SetATime(time.UnixMicro(1661073465))
	request.SetSize(29696)
	mFS.On("Chmod", "/test/happy_path", os.FileMode(0666)).Return(nil)
	mFS.On("Chown", "/test/happy_path", 0, 1).Return(nil)
	mFS.On("UtimesNano", "/test/happy_path", []syscall.Timespec{
		syscall.NsecToTimespec(time.UnixMicro(1661073465).UnixNano()),
		{Nsec: rpccommon.UTIME_OMIT},
	}).Return(nil)
	mFS.On("Truncate", "/test/happy_path", int64(29696)).Return(nil)
	mFI.On("Sys").Return(&syscall.Stat_t{
		Mode:    0660,
		Nlink:   1,
		Ino:     2929,
		Uid:     0,
		Gid:     1,
		Blocks:  29,
		Blksize: 1024,
		Size:    29696,
	})
	mFS.On("Lstat", "/test/happy_path").Return(&mFI, nil)
	mFS.On("Readlink", "/test/happy_path").Return("", nil)

	reply = rpccommon.SetAttrReply{}
	err = dfFSOps.SetAttr(request, &reply)

	assert.Equal(t, rpccommon.SetAttrReply{
		Mode:       0660,
		Nlink:      1,
		Ino:        2929,
		UID:        0,
		GID:        1,
		Size:       29696,
		Blocks:     29,
		Blksize:    1024,
		LinkTarget: "",
	}, reply)
	assert.NoError(t, err)
	mFI.AssertExpectations(t)
	mFS.AssertNotCalled(t, "Chmod", mock.Anything)
	mFS.AssertCalled(t, "Chown", "/test/happy_path", 0, 1)
	mFS.AssertCalled(t, "UtimesNano", "/test/happy_path", []syscall.Timespec{
		syscall.NsecToTimespec(time.UnixMicro(1661073465).UnixNano()),
		{Nsec: rpccommon.UTIME_OMIT},
	})
	mFS.AssertCalled(t, "Truncate", "/test/happy_path", int64(29696))
	mFS.AssertCalled(t, "Lstat", "/test/happy_path")
	mFS.AssertCalled(t, "Readlink", "/test/happy_path")

	// *** Testing error on fso.Stat()
	mFS = mockFS{}
	mFI = mockFileInfo{}
	request = rpccommon.SetAttrRequest{FullPath: "/test/error_on_stat"}
	request.SetMode(0666)
	request.SetUID(0)
	request.SetGID(1)
	request.SetATime(time.UnixMicro(1661073465))
	request.SetMTime(time.UnixMicro(1661073466))
	request.SetSize(29696)
	mFS.On("Chmod", "/test/error_on_stat", os.FileMode(0666)).Return(nil)
	mFS.On("Chown", "/test/error_on_stat", 0, 1).Return(nil)
	mFS.On("UtimesNano", "/test/error_on_stat", []syscall.Timespec{
		syscall.NsecToTimespec(time.UnixMicro(1661073465).UnixNano()),
		syscall.NsecToTimespec(time.UnixMicro(1661073466).UnixNano()),
	}).Return(nil)
	mFS.On("Truncate", "/test/error_on_stat", int64(29696)).Return(nil)
	mFI.On("Sys").Return(&syscall.Stat_t{
		Mode:    0666,
		Nlink:   1,
		Ino:     2929,
		Uid:     0,
		Gid:     1,
		Blocks:  29,
		Blksize: 1024,
		Size:    29696,
	})
	mFS.On("Lstat", "/test/error_on_stat").Return(&mFI, syscall.EINVAL)
	mFS.On("Readlink", "/test/error_on_stat").Return("", nil)

	reply = rpccommon.SetAttrReply{}
	err = dfFSOps.SetAttr(request, &reply)

	assert.Equal(t, rpccommon.SetAttrReply{}, reply)
	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("errno: EINVAL"), err)
	}
	mFI.AssertNotCalled(t, "Sys", mock.Anything)
	mFS.AssertCalled(t, "Chmod", "/test/error_on_stat", os.FileMode(0666))
	mFS.AssertCalled(t, "Chown", "/test/error_on_stat", 0, 1)
	mFS.AssertCalled(t, "UtimesNano", "/test/error_on_stat", []syscall.Timespec{
		syscall.NsecToTimespec(time.UnixMicro(1661073465).UnixNano()),
		syscall.NsecToTimespec(time.UnixMicro(1661073466).UnixNano()),
	})
	mFS.AssertCalled(t, "Truncate", "/test/error_on_stat", int64(29696))
	mFS.AssertCalled(t, "Lstat", "/test/error_on_stat")
	mFS.AssertNotCalled(t, "Readlink", mock.Anything)
}

func TestCloseAllFDs(t *testing.T) {
	dfFSOps := NewDockerFuseFSOps()

	f1 := mockFile{}
	f1.On("Close").Return(nil)
	f2 := mockFile{}
	f2.On("Close").Return(nil)
	f3 := mockFile{}
	f3.On("Close").Return(nil)
	dfFSOps.fds = map[uintptr]file{29: &f1, 30: &f2, 31: &f3}

	dfFSOps.CloseAllFDs()

	f1.AssertExpectations(t)
	f2.AssertExpectations(t)
	f3.AssertExpectations(t)
	assert.Equal(t, 0, len(dfFSOps.fds))
}
