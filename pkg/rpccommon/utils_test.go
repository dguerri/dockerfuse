package rpccommon

import (
	"errors"
	"io/fs"
	"syscall"
	"testing"
)

func TestFlagsConversion(t *testing.T) {
	sys := syscall.O_WRONLY | syscall.O_CREAT | syscall.O_TRUNC
	sa := SystemToSAFlags(sys)
	if sa == 0 {
		t.Fatalf("expected non zero")
	}
	got := SAFlagsToSystem(sa)
	if got != sys {
		t.Fatalf("expected %d got %d", sys, got)
	}
}

func TestErrnoSymConversion(t *testing.T) {
	if ErrnoToSym(syscall.EPERM) != "EPERM" {
		t.Fatalf("unexpected sym")
	}
	if SymToErrno("EPERM") != syscall.EPERM {
		t.Fatalf("unexpected errno")
	}
}

func TestErrnoToRPCErrorStringAndBack(t *testing.T) {
	err := ErrnoToRPCErrorString(&fs.PathError{Err: syscall.EACCES})
	if err == nil || err.Error() != "errno: EACCES" {
		t.Fatalf("unexpected %v", err)
	}
	if RPCErrorStringTOErrno(err) != syscall.EACCES {
		t.Fatalf("unexpected errno")
	}
}

func TestRPCErrorStringTOErrnoMalformed(t *testing.T) {
	if RPCErrorStringTOErrno(errors.New("bad")) != syscall.EIO {
		t.Fatalf("expected EIO")
	}
}

func TestErrnoToRPCErrorStringEOF(t *testing.T) {
	err := ErrnoToRPCErrorString(errors.New("EOF"))
	if err == nil || err.Error() != "EOF" {
		t.Fatalf("unexpected %v", err)
	}
}
