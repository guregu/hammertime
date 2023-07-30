package libc

import (
	"fmt"
	"unsafe"
)

type (
	Size = uint32 // size_t
	Ptr  = uint32 // int
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
