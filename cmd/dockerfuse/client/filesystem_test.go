package client

import (
	"os"
	"testing"
)

func TestOSFSReadFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/f"
	if err := os.WriteFile(path, []byte("hello"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := (&osFS{}).ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("got %s", data)
	}
}

func TestOSFSExecutable(t *testing.T) {
	exe, err := (&osFS{}).Executable()
	if err != nil {
		t.Fatalf("executable err: %v", err)
	}
	if exe == "" {
		t.Fatal("empty executable")
	}
}
