package hammertime

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go/v11"
	"github.com/guregu/hammertime/libc"
	"golang.org/x/exp/slices"
)

const ptrSize = libc.PtrSize

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
