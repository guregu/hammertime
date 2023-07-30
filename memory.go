package hammertime

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go/v11"
	"github.com/guregu/hammertime/libc"
	"golang.org/x/exp/slices"
)

type segfault struct {
	addr libc.Ptr
	max  libc.Ptr
}

func (sf segfault) Error() string {
	return fmt.Sprintf("segfault: %x > %x", sf.addr, sf.max)
}

func ensure(caller *wasmtime.Caller, fn func(base unsafe.Pointer, data []byte), addrs ...libc.Ptr) error {
	const overflow = 1 << 32

	mem := caller.GetExport("memory").Memory()
	defer runtime.KeepAlive(mem)
	base := mem.Data(caller)
	datasize := mem.DataSize(caller)
	maxphysaddr := libc.Size(datasize)
	if datasize >= overflow {
		return fmt.Errorf("memory is too big for wasm32: %d", datasize)
	}
	data := (*[overflow]byte)(base)[:datasize:datasize]

	maxaddr := slices.Max(addrs)
	if maxaddr > maxphysaddr {
		return segfault{addr: maxaddr, max: maxphysaddr}
	}

	fn(base, data)
	return nil
}
