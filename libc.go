package mywasi

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go/v11"
)

type int_t = int32
type uint_t = uint32
type size_t = uint_t
type ptr_t = uint_t

func ensure(caller *wasmtime.Caller, fn func(unsafe.Pointer), addrs ...int_t) error {
	mem := caller.GetExport("memory").Memory()
	defer runtime.KeepAlive(mem)
	data := mem.Data(caller)
	max := size_t(mem.DataSize(caller))
	for _, addr := range addrs {
		if size_t(addr) > max {
			return fmt.Errorf("segfault: %d > %d", addr, max)
		}
	}
	fn(data)
	return nil
}

func ensure2(caller *wasmtime.Caller, fn func(base unsafe.Pointer, data []byte), addrs ...int_t) error {
	mem := caller.GetExport("memory").Memory()
	defer runtime.KeepAlive(mem)
	base := mem.Data(caller)
	data := mem.UnsafeData(caller)
	max := size_t(mem.DataSize(caller))
	for _, addr := range addrs {
		if size_t(addr) > max {
			return fmt.Errorf("segfault: %d > %d", addr, max)
		}
	}
	fn(base, data)
	return nil
}

type ciovec struct {
	buf     ptr_t
	buf_len size_t
}

type iovec struct {
	buf     ptr_t
	buf_len size_t
}

type prestat_dir struct {
	tag     uint8
	dir_len size_t
}

type fdstat struct {
	/**
	 * File type.
	 */
	fs_filetype filetype_t
	/**
	 * File descriptor flags.
	 */
	fs_flags fdflag_t
	/**
	 * Rights that apply to this file descriptor.
	 */
	fs_rights_base uint64
	/**
	 * Maximum set of rights that may be installed on new file descriptors that
	 * are created through this file descriptor, e.g., through `path_open`.
	 */
	fs_rights_inheriting uint64
}

// func transmute[T any](store wasmtime.Storelike, p ptr_t) T {
// 	data := store.Data()
// }

type filetype_t = uint8

const (
	// The type of a file descriptor or file is unknown or is different from any of the other types specified.
	filetypeUnknown filetype_t = 0
	// The file descriptor or file refers to a block device inode.
	filetypeBlockDevice filetype_t = 1
	// The file descriptor or file refers to a character device inode.
	filetypeCharacterDevice filetype_t = 2
	// The file descriptor or file refers to a directory inode.
	filetypeDirectory filetype_t = 3
	// The file descriptor or file refers to a regular file inode.
	filetypeRegularFile filetype_t = 4
	// The file descriptor or file refers to a datagram socket.
	filetypeSocketDgram filetype_t = 5
	// The file descriptor or file refers to a byte-stream socket.
	filetypeSocketStream filetype_t = 6
	// The file refers to a symbolic link inode.
	filetypeSymbolicLink filetype_t = 7
)

type fdflag_t uint16

const (
	// Append mode: Data written to the file is always appended to the file's end.
	fdflagAppend fdflag_t = 1 << 0
	// Write according to synchronized I/O data integrity completion. Only the data stored in the file is synchronized.
	fdflagDSync fdflag_t = 1 << 1
	// Non-blocking mode.
	fdflagNonBlock fdflag_t = 1 << 2
	// Synchronized read I/O operations.
	fdflagRSync fdflag_t = 1 << 3
	// Write according to synchronized I/O file integrity completion. In addition to synchronizing the data stored in the file, the implementation may also synchronously update the file's metadata.
	fdflagSync fdflag_t = 1 << 4
)
