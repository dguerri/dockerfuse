package rpc_common

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"reflect"
	"strings"
	"syscall"
)

// open() flags. Different OSes use different values for open flags
const (
	O_RDONLY   uint16 = 0b0000000000000000
	O_WRONLY   uint16 = 0b0000000000000001
	O_RDWR     uint16 = 0b0000000000000010
	O_APPEND   uint16 = 0b0000000000000100
	O_ASYNC    uint16 = 0b0000000000001000
	O_CREAT    uint16 = 0b0000000000010000
	O_EXCL     uint16 = 0b0000000000100000
	O_NOCTTY   uint16 = 0b0000000001000000
	O_NONBLOCK uint16 = 0b0000000010000000
	O_SYNC     uint16 = 0b0000000100000000
	O_TRUNC    uint16 = 0b0000001000000000
)

/* Converts system-specific open() flags to an internal representation.
   See also SAFlagsToSystem() */
func SystemToSAFlags(flags_in int) (flags uint16) {
	if flags_in == syscall.O_RDONLY {
		return O_RDONLY
	}
	if flags_in&syscall.O_WRONLY == syscall.O_WRONLY {
		flags |= O_WRONLY
	}
	if flags_in&syscall.O_RDWR == syscall.O_RDWR {
		flags |= O_RDWR
	}
	if flags_in&syscall.O_APPEND == syscall.O_APPEND {
		flags |= O_APPEND
	}
	if flags_in&syscall.O_ASYNC == syscall.O_ASYNC {
		flags |= O_ASYNC
	}
	if flags_in&syscall.O_CREAT == syscall.O_CREAT {
		flags |= O_CREAT
	}
	if flags_in&syscall.O_EXCL == syscall.O_EXCL {
		flags |= O_EXCL
	}
	if flags_in&syscall.O_NOCTTY == syscall.O_NOCTTY {
		flags |= O_NOCTTY
	}
	if flags_in&syscall.O_NONBLOCK == syscall.O_NONBLOCK {
		flags |= O_NONBLOCK
	}
	if flags_in&syscall.O_SYNC == syscall.O_SYNC {
		flags |= O_SYNC
	}
	if flags_in&syscall.O_TRUNC == syscall.O_TRUNC {
		flags |= O_TRUNC
	}
	return
}

/* Converts the internal representation of open() flags to system-specific.
   See also SystemToSAFlags() */
func SAFlagsToSystem(flags_in uint16) (flags int) {
	if flags_in == O_RDONLY {
		return syscall.O_RDONLY // O_RDONLY == 0
	}
	if flags_in&O_WRONLY == O_WRONLY {
		flags |= syscall.O_WRONLY
	}
	if flags_in&O_RDWR == O_RDWR {
		flags |= syscall.O_RDWR
	}
	if flags_in&O_APPEND == O_APPEND {
		flags |= syscall.O_APPEND
	}
	if flags_in&O_ASYNC == O_ASYNC {
		flags |= syscall.O_ASYNC
	}
	if flags_in&O_CREAT == O_CREAT {
		flags |= syscall.O_CREAT
	}
	if flags_in&O_EXCL == O_EXCL {
		flags |= syscall.O_EXCL
	}
	if flags_in&O_NOCTTY == O_NOCTTY {
		flags |= syscall.O_NOCTTY
	}
	if flags_in&O_NONBLOCK == O_NONBLOCK {
		flags |= syscall.O_NONBLOCK
	}
	if flags_in&O_SYNC == O_SYNC {
		flags |= syscall.O_SYNC
	}
	if flags_in&O_TRUNC == O_TRUNC {
		flags |= syscall.O_TRUNC
	}
	return
}

/* Converts system-specific errno codes to strings, to be converted back to
   errno codes on the other side.
	 This is needed as errno codes are not portable. (See Linux ENOTEMPTY vs
	 Darwin EDESTADDRREQ). See also SymToErrno().
*/
func ErrnoToSym(errno syscall.Errno) string {
	switch errno {
	case syscall.E2BIG:
		return "E2BIG"
	case syscall.EACCES:
		return "EACCES"
	case syscall.EADDRINUSE:
		return "EADDRINUSE"
	case syscall.EADDRNOTAVAIL:
		return "EADDRNOTAVAIL"
	case syscall.EAFNOSUPPORT:
		return "EAFNOSUPPORT"
	case syscall.EAGAIN:
		return "EAGAIN"
	case syscall.EALREADY:
		return "EALREADY"
	case syscall.EBADF:
		return "EBADF"
	case syscall.EBADMSG:
		return "EBADMSG"
	case syscall.EBUSY:
		return "EBUSY"
	case syscall.ECANCELED:
		return "ECANCELED"
	case syscall.ECHILD:
		return "ECHILD"
	case syscall.ECONNABORTED:
		return "ECONNABORTED"
	case syscall.ECONNREFUSED:
		return "ECONNREFUSED"
	case syscall.ECONNRESET:
		return "ECONNRESET"
	case syscall.EDEADLK:
		return "EDEADLK"
	case syscall.EDESTADDRREQ:
		return "EDESTADDRREQ"
	case syscall.EDOM:
		return "EDOM"
	case syscall.EDQUOT:
		return "EDQUOT"
	case syscall.EEXIST:
		return "EEXIST"
	case syscall.EFAULT:
		return "EFAULT"
	case syscall.EFBIG:
		return "EFBIG"
	case syscall.EHOSTDOWN:
		return "EHOSTDOWN"
	case syscall.EHOSTUNREACH:
		return "EHOSTUNREACH"
	case syscall.EIDRM:
		return "EIDRM"
	case syscall.EILSEQ:
		return "EILSEQ"
	case syscall.EINPROGRESS:
		return "EINPROGRESS"
	case syscall.EINTR:
		return "EINTR"
	case syscall.EINVAL:
		return "EINVAL"
	case syscall.EIO:
		return "EIO"
	case syscall.EISCONN:
		return "EISCONN"
	case syscall.EISDIR:
		return "EISDIR"
	case syscall.ELOOP:
		return "ELOOP"
	case syscall.EMFILE:
		return "EMFILE"
	case syscall.EMLINK:
		return "EMLINK"
	case syscall.EMSGSIZE:
		return "EMSGSIZE"
	case syscall.EMULTIHOP:
		return "EMULTIHOP"
	case syscall.ENAMETOOLONG:
		return "ENAMETOOLONG"
	case syscall.ENETDOWN:
		return "ENETDOWN"
	case syscall.ENETRESET:
		return "ENETRESET"
	case syscall.ENETUNREACH:
		return "ENETUNREACH"
	case syscall.ENFILE:
		return "ENFILE"
	case syscall.ENOBUFS:
		return "ENOBUFS"
	case syscall.ENODATA:
		return "ENODATA"
	case syscall.ENODEV:
		return "ENODEV"
	case syscall.ENOENT:
		return "ENOENT"
	case syscall.ENOEXEC:
		return "ENOEXEC"
	case syscall.ENOLCK:
		return "ENOLCK"
	case syscall.ENOLINK:
		return "ENOLINK"
	case syscall.ENOMEM:
		return "ENOMEM"
	case syscall.ENOMSG:
		return "ENOMSG"
	case syscall.ENOPROTOOPT:
		return "ENOPROTOOPT"
	case syscall.ENOSPC:
		return "ENOSPC"
	case syscall.ENOSR:
		return "ENOSR"
	case syscall.ENOSTR:
		return "ENOSTR"
	case syscall.ENOSYS:
		return "ENOSYS"
	case syscall.ENOTBLK:
		return "ENOTBLK"
	case syscall.ENOTCONN:
		return "ENOTCONN"
	case syscall.ENOTDIR:
		return "ENOTDIR"
	case syscall.ENOTEMPTY:
		return "ENOTEMPTY"
	case syscall.ENOTRECOVERABLE:
		return "ENOTRECOVERABLE"
	case syscall.ENOTSOCK:
		return "ENOTSOCK"
	case syscall.ENOTSUP:
		return "ENOTSUP"
	case syscall.ENOTTY:
		return "ENOTTY"
	case syscall.ENXIO:
		return "ENXIO"
	case syscall.EOVERFLOW:
		return "EOVERFLOW"
	case syscall.EOWNERDEAD:
		return "EOWNERDEAD"
	case syscall.EPERM:
		return "EPERM"
	case syscall.EPFNOSUPPORT:
		return "EPFNOSUPPORT"
	case syscall.EPIPE:
		return "EPIPE"
	case syscall.EPROTO:
		return "EPROTO"
	case syscall.EPROTONOSUPPORT:
		return "EPROTONOSUPPORT"
	case syscall.EPROTOTYPE:
		return "EPROTOTYPE"
	case syscall.ERANGE:
		return "ERANGE"
	case syscall.EREMOTE:
		return "EREMOTE"
	case syscall.EROFS:
		return "EROFS"
	case syscall.ESHUTDOWN:
		return "ESHUTDOWN"
	case syscall.ESOCKTNOSUPPORT:
		return "ESOCKTNOSUPPORT"
	case syscall.ESPIPE:
		return "ESPIPE"
	case syscall.ESRCH:
		return "ESRCH"
	case syscall.ESTALE:
		return "ESTALE"
	case syscall.ETIME:
		return "ETIME"
	case syscall.ETIMEDOUT:
		return "ETIMEDOUT"
	case syscall.ETOOMANYREFS:
		return "ETOOMANYREFS"
	case syscall.ETXTBSY:
		return "ETXTBSY"
	case syscall.EUSERS:
		return "EUSERS"
	case syscall.EXDEV:
		return "EXDEV"
	default:
		return "EIO"
	}
}

/* Convert symbolic representaton of an errno to errno code.
   See also ErrnoToSym().
*/
func SymToErrno(sym string) syscall.Errno {
	switch sym {
	case "E2BIG":
		return syscall.E2BIG
	case "EACCES":
		return syscall.EACCES
	case "EADDRINUSE":
		return syscall.EADDRINUSE
	case "EADDRNOTAVAIL":
		return syscall.EADDRNOTAVAIL
	case "EAFNOSUPPORT":
		return syscall.EAFNOSUPPORT
	case "EAGAIN":
		return syscall.EAGAIN
	case "EALREADY":
		return syscall.EALREADY
	case "EBADF":
		return syscall.EBADF
	case "EBADMSG":
		return syscall.EBADMSG
	case "EBUSY":
		return syscall.EBUSY
	case "ECANCELED":
		return syscall.ECANCELED
	case "ECHILD":
		return syscall.ECHILD
	case "ECONNABORTED":
		return syscall.ECONNABORTED
	case "ECONNREFUSED":
		return syscall.ECONNREFUSED
	case "ECONNRESET":
		return syscall.ECONNRESET
	case "EDEADLK":
		return syscall.EDEADLK
	case "EDESTADDRREQ":
		return syscall.EDESTADDRREQ
	case "EDOM":
		return syscall.EDOM
	case "EDQUOT":
		return syscall.EDQUOT
	case "EEXIST":
		return syscall.EEXIST
	case "EFAULT":
		return syscall.EFAULT
	case "EFBIG":
		return syscall.EFBIG
	case "EHOSTDOWN":
		return syscall.EHOSTDOWN
	case "EHOSTUNREACH":
		return syscall.EHOSTUNREACH
	case "EIDRM":
		return syscall.EIDRM
	case "EILSEQ":
		return syscall.EILSEQ
	case "EINPROGRESS":
		return syscall.EINPROGRESS
	case "EINTR":
		return syscall.EINTR
	case "EINVAL":
		return syscall.EINVAL
	case "EIO":
		return syscall.EIO
	case "EISCONN":
		return syscall.EISCONN
	case "EISDIR":
		return syscall.EISDIR
	case "ELOOP":
		return syscall.ELOOP
	case "EMFILE":
		return syscall.EMFILE
	case "EMLINK":
		return syscall.EMLINK
	case "EMSGSIZE":
		return syscall.EMSGSIZE
	case "EMULTIHOP":
		return syscall.EMULTIHOP
	case "ENAMETOOLONG":
		return syscall.ENAMETOOLONG
	case "ENETDOWN":
		return syscall.ENETDOWN
	case "ENETRESET":
		return syscall.ENETRESET
	case "ENETUNREACH":
		return syscall.ENETUNREACH
	case "ENFILE":
		return syscall.ENFILE
	case "ENOBUFS":
		return syscall.ENOBUFS
	case "ENODATA":
		return syscall.ENODATA
	case "ENODEV":
		return syscall.ENODEV
	case "ENOENT":
		return syscall.ENOENT
	case "ENOEXEC":
		return syscall.ENOEXEC
	case "ENOLCK":
		return syscall.ENOLCK
	case "ENOLINK":
		return syscall.ENOLINK
	case "ENOMEM":
		return syscall.ENOMEM
	case "ENOMSG":
		return syscall.ENOMSG
	case "ENOPROTOOPT":
		return syscall.ENOPROTOOPT
	case "ENOSPC":
		return syscall.ENOSPC
	case "ENOSR":
		return syscall.ENOSR
	case "ENOSTR":
		return syscall.ENOSTR
	case "ENOSYS":
		return syscall.ENOSYS
	case "ENOTBLK":
		return syscall.ENOTBLK
	case "ENOTCONN":
		return syscall.ENOTCONN
	case "ENOTDIR":
		return syscall.ENOTDIR
	case "ENOTEMPTY":
		return syscall.ENOTEMPTY
	case "ENOTRECOVERABLE":
		return syscall.ENOTRECOVERABLE
	case "ENOTSOCK":
		return syscall.ENOTSOCK
	case "ENOTSUP":
		return syscall.ENOTSUP
	case "ENOTTY":
		return syscall.ENOTTY
	case "ENXIO":
		return syscall.ENXIO
	case "EOPNOTSUPP":
		return syscall.EOPNOTSUPP
	case "EOVERFLOW":
		return syscall.EOVERFLOW
	case "EOWNERDEAD":
		return syscall.EOWNERDEAD
	case "EPERM":
		return syscall.EPERM
	case "EPFNOSUPPORT":
		return syscall.EPFNOSUPPORT
	case "EPIPE":
		return syscall.EPIPE
	case "EPROTO":
		return syscall.EPROTO
	case "EPROTONOSUPPORT":
		return syscall.EPROTONOSUPPORT
	case "EPROTOTYPE":
		return syscall.EPROTOTYPE
	case "ERANGE":
		return syscall.ERANGE
	case "EREMOTE":
		return syscall.EREMOTE
	case "EROFS":
		return syscall.EROFS
	case "ESHUTDOWN":
		return syscall.ESHUTDOWN
	case "ESOCKTNOSUPPORT":
		return syscall.ESOCKTNOSUPPORT
	case "ESPIPE":
		return syscall.ESPIPE
	case "ESRCH":
		return syscall.ESRCH
	case "ESTALE":
		return syscall.ESTALE
	case "ETIME":
		return syscall.ETIME
	case "ETIMEDOUT":
		return syscall.ETIMEDOUT
	case "ETOOMANYREFS":
		return syscall.ETOOMANYREFS
	case "ETXTBSY":
		return syscall.ETXTBSY
	case "EUSERS":
		return syscall.EUSERS
	case "EWOULDBLOCK":
		return syscall.EWOULDBLOCK
	case "EXDEV":
		return syscall.EXDEV
	default:
		return syscall.EIO
	}
}

func ErrnoToRPCErrorString(err error) error {
	switch e := err.(type) {
	case *fs.PathError:
		log.Printf("errno: %s", ErrnoToSym(e.Err.(syscall.Errno)))
		return fmt.Errorf("errno: %s", ErrnoToSym(e.Err.(syscall.Errno)))
	case *os.LinkError:
		log.Printf("errno: %s", ErrnoToSym(e.Err.(syscall.Errno)))
		return fmt.Errorf("errno: %s", ErrnoToSym(e.Err.(syscall.Errno)))
	case syscall.Errno:
		log.Printf("errno: %s", ErrnoToSym(e))
		return fmt.Errorf("errno: %s", ErrnoToSym(e))
	default:
		if err.Error() == "EOF" {
			log.Print("EOF")
			return err
		}
		log.Printf("unknown error: %s (%s)", err.Error(), reflect.TypeOf(err))
		return fmt.Errorf("unknown error: %s", err.Error())
	}
}

func RPCErrorStringTOErrno(err error) (syserr syscall.Errno) {
	if strings.HasPrefix(err.Error(), "errno: ") {
		return SymToErrno(strings.SplitN(err.Error(), " ", 2)[1])
	}
	log.Printf("malformed error from server: %s", err.Error())
	syserr = syscall.EIO
	return
}
