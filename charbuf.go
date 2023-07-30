package hammertime

import (
	"bytes"
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go/v11"

	"github.com/guregu/hammertime/libc"
)

type charbuffer []string

func (strs charbuffer) size() libc.Size {
	var size libc.Size
	for _, arg := range strs {
		size += libc.Size(len(arg) + 1)
	}
	return size
}

func (strs charbuffer) writeSizes(caller *wasmtime.Caller, _argc, _argv int32) error {
	argc := libc.Ptr(_argc)
	argv := libc.Ptr(_argv)
	return ensure(caller, func(base unsafe.Pointer, _ []byte) {
		*(*uint32)(unsafe.Add(base, argc)) = uint32(len(strs))
		*(*uint32)(unsafe.Add(base, argv)) = strs.size()
	}, argc+libc.PtrSize, argv+libc.PtrSize)
}

func (strs charbuffer) write(caller *wasmtime.Caller, _listptr, _bufptr int32) error {
	listptr := libc.Ptr(_listptr)
	bufptr := libc.Ptr(_bufptr)
	return ensure(caller, func(base unsafe.Pointer, data []byte) {
		var buf bytes.Buffer
		list0 := (*libc.Ptr)(unsafe.Add(base, listptr))
		list := unsafe.Slice(list0, len(strs))
		for i, arg := range strs {
			ptr := libc.Ptr(buf.Len()) + bufptr
			list[i] = ptr
			buf.WriteString(arg)
			buf.WriteByte(0)
		}
		copy(data[bufptr:], buf.Bytes())
	}, listptr+libc.PtrSize*libc.Ptr(len(strs)), bufptr+strs.size())
}
