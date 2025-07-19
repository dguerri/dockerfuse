package rpccommon

import (
	"testing"
	"time"
)

func TestSetAttrRequestSettersGetters(t *testing.T) {
	var r SetAttrRequest
	tm := time.Unix(10, 0)
	r.SetMode(0755)
	r.SetUID(1)
	r.SetGID(2)
	r.SetATime(tm)
	r.SetMTime(tm)
	r.SetSize(64)

	if m, ok := r.GetMode(); !ok || m != 0755 {
		t.Fatalf("mode")
	}
	if u, ok := r.GetUID(); !ok || u != 1 {
		t.Fatalf("uid")
	}
	if g, ok := r.GetGID(); !ok || g != 2 {
		t.Fatalf("gid")
	}
	if a, ok := r.GetATime(); !ok || !a.Equal(tm) {
		t.Fatalf("atime")
	}
	if mtime, ok := r.GetMTime(); !ok || !mtime.Equal(tm) {
		t.Fatalf("mtime")
	}
	if s, ok := r.GetSize(); !ok || s != 64 {
		t.Fatalf("size")
	}
}

func TestSetAttrRequestUnset(t *testing.T) {
	var r SetAttrRequest
	if _, ok := r.GetMode(); ok {
		t.Fatal("mode set")
	}
	if _, ok := r.GetUID(); ok {
		t.Fatal("uid set")
	}
	if _, ok := r.GetGID(); ok {
		t.Fatal("gid set")
	}
	if _, ok := r.GetATime(); ok {
		t.Fatal("atime set")
	}
	if _, ok := r.GetMTime(); ok {
		t.Fatal("mtime set")
	}
	if _, ok := r.GetSize(); ok {
		t.Fatal("size set")
	}
}
