package server

import (
	"os"
	"syscall"
	"testing"
	"time"
)

// Test basic behaviour of osFS implementation
func TestOSFSOperations(t *testing.T) {
	fs := &osFS{}
	dir := t.TempDir()

	// Mkdir
	subdir := dir + "/sub"
	if err := fs.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("mkdir error: %v", err)
	}
	if fi, err := os.Stat(subdir); err != nil || !fi.IsDir() {
		t.Fatalf("subdir missing or not dir: %v", err)
	}

	// OpenFile + Write
	fpath := dir + "/file"
	f, err := fs.OpenFile(fpath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("openfile error: %v", err)
	}
	if _, err := f.Write([]byte("data")); err != nil {
		t.Fatalf("write error: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}
	if _, err := fs.Lstat(fpath); err != nil {
		t.Fatalf("lstat error: %v", err)
	}

	// ReadDir should list created entries
	entries, err := fs.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no entries read")
	}

	// Symlink and Readlink
	link := dir + "/link"
	if err := fs.Symlink(fpath, link); err != nil {
		t.Fatalf("symlink error: %v", err)
	}
	target, err := fs.Readlink(link)
	if err != nil || target != fpath {
		t.Fatalf("readlink mismatch: %v %s", err, target)
	}

	// Truncate
	if err := fs.Truncate(fpath, 1); err != nil {
		t.Fatalf("truncate error: %v", err)
	}
	if fi, _ := os.Stat(fpath); fi.Size() != 1 {
		t.Fatalf("unexpected size %d", fi.Size())
	}

	// UtimesNano
	ts := []syscall.Timespec{
		syscall.NsecToTimespec(time.Unix(1, 0).UnixNano()),
		syscall.NsecToTimespec(time.Unix(2, 0).UnixNano()),
	}
	if err := fs.UtimesNano(fpath, ts); err != nil {
		t.Fatalf("utimes error: %v", err)
	}

	// Rename and Link
	renamed := dir + "/renamed"
	if err := fs.Rename(fpath, renamed); err != nil {
		t.Fatalf("rename error: %v", err)
	}
	hard := dir + "/hard"
	if err := fs.Link(renamed, hard); err != nil {
		t.Fatalf("link error: %v", err)
	}

	// Chmod and Chown (use current uid/gid)
	if err := fs.Chmod(renamed, 0o644); err != nil {
		t.Fatalf("chmod error: %v", err)
	}
	if err := fs.Chown(renamed, os.Getuid(), os.Getgid()); err != nil {
		t.Fatalf("chown error: %v", err)
	}

	// Remove
	if err := fs.Remove(hard); err != nil {
		t.Fatalf("remove error: %v", err)
	}
}
