package rpccommon

import (
	"errors"
	"io/fs"
	"os"
	"strings"
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

func TestFlagsConversionExtended(t *testing.T) {
	sys := syscall.O_RDWR | syscall.O_APPEND | syscall.O_ASYNC | syscall.O_EXCL |
		syscall.O_NOCTTY | syscall.O_NONBLOCK | syscall.O_SYNC | syscall.O_CREAT
	sa := SystemToSAFlags(sys)
	if sa&O_RDWR == 0 || sa&O_APPEND == 0 || sa&O_ASYNC == 0 || sa&O_EXCL == 0 ||
		sa&O_NOCTTY == 0 || sa&O_NONBLOCK == 0 || sa&O_SYNC == 0 {
		t.Fatalf("missing flags in sa %d", sa)
	}
	got := SAFlagsToSystem(sa)
	if got != sys {
		t.Fatalf("expected %d got %d", sys, got)
	}
}

func TestFlagsConversionReadOnly(t *testing.T) {
	if SystemToSAFlags(syscall.O_RDONLY) != O_RDONLY {
		t.Fatalf("readonly conversion failed")
	}
	if SAFlagsToSystem(O_RDONLY) != syscall.O_RDONLY {
		t.Fatalf("system readonly conversion failed")
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

func TestErrnoSymConversionAll(t *testing.T) {
	symbols := []string{
		"E2BIG", "EACCES", "EADDRINUSE", "EADDRNOTAVAIL", "EAFNOSUPPORT", "EAGAIN", "EALREADY",
		"EBADF", "EBADMSG", "EBUSY", "ECANCELED", "ECHILD", "ECONNABORTED", "ECONNREFUSED",
		"ECONNRESET", "EDEADLK", "EDESTADDRREQ", "EDOM", "EDQUOT", "EEXIST", "EFAULT", "EFBIG",
		"EHOSTDOWN", "EHOSTUNREACH", "EIDRM", "EILSEQ", "EINPROGRESS", "EINTR", "EINVAL", "EIO",
		"EISCONN", "EISDIR", "ELOOP", "EMFILE", "EMLINK", "EMSGSIZE", "EMULTIHOP", "ENAMETOOLONG",
		"ENETDOWN", "ENETRESET", "ENETUNREACH", "ENFILE", "ENOBUFS", "ENODATA", "ENODEV", "ENOENT",
		"ENOEXEC", "ENOLCK", "ENOLINK", "ENOMEM", "ENOMSG", "ENOPROTOOPT", "ENOSPC", "ENOSR",
		"ENOSTR", "ENOSYS", "ENOTBLK", "ENOTCONN", "ENOTDIR", "ENOTEMPTY", "ENOTRECOVERABLE",
		"ENOTSOCK", "ENOTSUP", "ENOTTY", "ENXIO", "EOVERFLOW", "EOWNERDEAD", "EPERM", "EPFNOSUPPORT",
		"EPIPE", "EPROTO", "EPROTONOSUPPORT", "EPROTOTYPE", "ERANGE", "EREMOTE", "EROFS", "ESHUTDOWN",
		"ESOCKTNOSUPPORT", "ESPIPE", "ESRCH", "ESTALE", "ETIME", "ETIMEDOUT", "ETOOMANYREFS",
		"ETXTBSY", "EUSERS", "EXDEV",
		// Add special cases, these differes based on the platform
		// "EWOULDBLOCK", "EOPNOTSUPP",
	}
	for _, sym := range symbols {
		errno := SymToErrno(sym)
		got := ErrnoToSym(errno)
		expected := sym
		if got != expected {
			t.Fatalf("conversion mismatch for %s => %s", sym, got)
		}
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

func TestErrnoToRPCErrorStringLinkError(t *testing.T) {
	err := ErrnoToRPCErrorString(&os.LinkError{Err: syscall.EEXIST})
	if err == nil || err.Error() != "errno: EEXIST" {
		t.Fatalf("unexpected %v", err)
	}
}

func TestErrnoToRPCErrorStringSyscallErrno(t *testing.T) {
	err := ErrnoToRPCErrorString(syscall.ENOTDIR)
	if err == nil || err.Error() != "errno: ENOTDIR" {
		t.Fatalf("unexpected %v", err)
	}
}

func TestErrnoToRPCErrorStringUnknown(t *testing.T) {
	e := errors.New("something")
	err := ErrnoToRPCErrorString(e)
	if err == nil || !strings.HasPrefix(err.Error(), "unknown error: ") {
		t.Fatalf("unexpected %v", err)
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
