package mywasi

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go/v11"
	"golang.org/x/exp/slices"
)

const ptrSize = 4

type int_t = int32
type uint_t = uint32
type size_t = uint_t
type ptr_t = uint_t

type segfault struct {
	addr ptr_t
	max  ptr_t
}

func (sf segfault) Error() string {
	return fmt.Sprintf("segfault: %x > %x", sf.addr, sf.max)
}

func ensure(caller *wasmtime.Caller, fn func(base unsafe.Pointer, data []byte), addrs ...ptr_t) error {
	mem := caller.GetExport("memory").Memory()
	defer runtime.KeepAlive(mem)
	base := mem.Data(caller)
	data := mem.UnsafeData(caller)
	maxphysaddr := size_t(mem.DataSize(caller))
	maxaddr := slices.Max(addrs)
	if maxaddr > maxphysaddr {
		return segfault{addr: maxaddr, max: maxphysaddr}
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
	fs_filetype          filetype_t
	fs_flags             fdflag_t
	fs_rights_base       uint64
	fs_rights_inheriting uint64
}

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
	fdflagAppend fdflag_t = 1 << iota
	// Write according to synchronized I/O data integrity completion. Only the data stored in the file is synchronized.
	fdflagDSync
	// Non-blocking mode.
	fdflagNonBlock
	// Synchronized read I/O operations.
	fdflagRSync
	// Write according to synchronized I/O file integrity completion. In addition to synchronizing the data stored in the file, the implementation may also synchronously update the file's metadata.
	fdflagSync
)

type filestat struct {
	// Device ID of device containing the file.
	dev uint64
	// File serial number.
	ino uint64
	// File type.
	filetype filetype_t
	// Number of hard links to the file.
	nlink uint64
	// For regular files, the file size in bytes. For symbolic links, the length in bytes of the pathname contained in the symbolic link.
	size uint64
	// Last data access timestamp.
	atim uint64
	// Last data modification timestamp.
	mtim uint64
	// Last file status change timestamp.
	ctim uint64
}

type dirent struct {
	next   uint64
	ino    uint64
	namlen size_t
	dtype  uint8
}

func init() {
	// ensure size of structs used for transmute match wit sizes
	assert := func(name string, size uintptr, want uintptr) {
		if size != want {
			panic(fmt.Errorf("sizeof(%s) != %d (got %d)", name, want, size))
		}
	}
	assert("iovec", unsafe.Sizeof(iovec{}), 8)
	assert("ciovec", unsafe.Sizeof(ciovec{}), 8)
	assert("fdstat", unsafe.Sizeof(fdstat{}), 24)
	assert("filestat", unsafe.Sizeof(filestat{}), 64)
	assert("dirent", unsafe.Sizeof(dirent{}), 24)
	assert("i64", unsafe.Sizeof(int64(0)), 8)
}
