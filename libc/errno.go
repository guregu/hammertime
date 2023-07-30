package libc

import (
	"errors"
	"io/fs"
	"log"
)

type Errno = int32

const (
	ErrnoSuccess        Errno = iota // No error occurred. System call completed successfully.
	Errno2big                        // Argument list too long.
	ErrnoAcces                       // Permission denied.
	ErrnoAddrinuse                   // Address in use.
	ErrnoAddrnotavail                // Address not available.
	ErrnoAfnosupport                 // Address family not supported.
	ErrnoAgain                       // Resource unavailable, or operation would block.
	ErrnoAlready                     // Connection already in progress.
	ErrnoBadf                        // Bad file descriptor.
	ErrnoBadmsg                      // Bad message.
	ErrnoBusy                        // Device or resource busy.
	ErrnoCanceled                    // Operation canceled.
	ErrnoChild                       // No child processes.
	ErrnoConnaborted                 // Connection aborted.
	ErrnoConnrefused                 // Connection refused.
	ErrnoConnreset                   // Connection reset.
	ErrnoDeadlk                      // Resource deadlock would occur.
	ErrnoDestaddrreq                 // Destination address required.
	ErrnoDom                         // Mathematics argument out of domain of function.
	ErrnoDquot                       // Reserved.
	ErrnoExist                       // File exists.
	ErrnoFault                       // Bad address.
	ErrnoFbig                        // File too large.
	ErrnoHostunreach                 // Host is unreachable.
	ErrnoIdrm                        // Identifier removed.
	ErrnoIlseq                       // Illegal byte sequence.
	ErrnoInprogress                  // Operation in progress.
	ErrnoIntr                        // Interrupted function.
	ErrnoInval                       // Invalid argument.
	ErrnoIo                          // I/O error.
	ErrnoIsconn                      // Socket is connected.
	ErrnoIsdir                       // Is a directory.
	ErrnoLoop                        // Too many levels of symbolic links.
	ErrnoMfile                       // File descriptor value too large.
	ErrnoMlink                       // Too many links.
	ErrnoMsgsize                     // Message too large.
	ErrnoMultihop                    // Reserved.
	ErrnoNametoolong                 // Filename too long.
	ErrnoNetdown                     // Network is down.
	ErrnoNetreset                    // Connection aborted by network.
	ErrnoNetunreach                  // Network unreachable.
	ErrnoNfile                       // Too many files open in system.
	ErrnoNobufs                      // No buffer space available.
	ErrnoNodev                       // No such device.
	ErrnoNoent                       // No such file or directory.
	ErrnoNoexec                      // Executable file format error.
	ErrnoNolck                       // No locks available.
	ErrnoNolink                      // Reserved.
	ErrnoNomem                       // Not enough space.
	ErrnoNomsg                       // No message of the desired type.
	ErrnoNoprotoopt                  // Protocol not available.
	ErrnoNospc                       // No space left on device.
	ErrnoNosys                       // Function not supported.
	ErrnoNotconn                     // The socket is not connected.
	ErrnoNotdir                      // Not a directory or a symbolic link to a directory.
	ErrnoNotempty                    // Directory not empty.
	ErrnoNotrecoverable              // State not recoverable.
	ErrnoNotsock                     // Not a socket.
	ErrnoNotsup                      // Not supported, or operation not supported on socket.
	ErrnoNotty                       // Inappropriate I/O control operation.
	ErrnoNxio                        // No such device or address.
	ErrnoOverflow                    // Value too large to be stored in data type.
	ErrnoOwnerdead                   // Previous owner died.
	ErrnoPerm                        // Operation not permitted.
	ErrnoPipe                        // Broken pipe.
	ErrnoProto                       // Protocol error.
	ErrnoProtonosupport              // Protocol not supported.
	ErrnoPrototype                   // Protocol wrong type for socket.
	ErrnoRange                       // Result too large.
	ErrnoRofs                        // Read-only file system.
	ErrnoSpipe                       // Invalid seek.
	ErrnoSrch                        // No such process.
	ErrnoStale                       // Reserved.
	ErrnoTimedout                    // Connection timed out.
	ErrnoTxtbsy                      // Text file busy.
	ErrnoXdev                        // Cross-device link.
	ErrnoNotcapable                  // Extension: Capabilities insufficient.
)

func Error(err error) Errno {
	switch {
	case err == nil:
		return ErrnoSuccess
	case errors.Is(err, fs.ErrExist):
		return ErrnoNoent
	case errors.Is(err, fs.ErrInvalid):
		return ErrnoInval
	}
	log.Println("unhandled errno error:", err)
	return ErrnoNosys
}
