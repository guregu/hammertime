package libc

import (
	"fmt"
	"unsafe"
)

// Maybe if we're careful enough here we could convert to wasm64 eventually?

type (
	Int  = int32  // int
	Uint = uint32 // unsigned int

	Size    = uint32 // size_t
	Ssize   = int32  // ssize_t
	Ptr     = uint32 // uintptr_t
	Ptrdiff = int32  // ptrdiff_t
)

const PtrSize = 4

func init() {
	// ensure size of structs used for transmute match wit sizes
	assert := func(name string, size uintptr, want uintptr) {
		if size != want {
			panic(fmt.Errorf("sizeof(%s) != %d (got %d)", name, want, size))
		}
	}
	assert("iovec", unsafe.Sizeof(Iovec{}), 8)
	assert("ciovec", unsafe.Sizeof(Ciovec{}), 8)
	assert("prestat_dir", unsafe.Sizeof(PrestatDir{}), 8)
	assert("fdstat", unsafe.Sizeof(Fdstat{}), 24)
	assert("filestat", unsafe.Sizeof(Filestat{}), 64)
	assert("dirent", unsafe.Sizeof(Dirent{}), 24)
	assert("i64", unsafe.Sizeof(int64(0)), 8)
}
