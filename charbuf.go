package hammertime

import (
	"bytes"
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go/v11"
)

type charbuffer []string

func (strs charbuffer) size() size_t {
	var size size_t
	for _, arg := range strs {
		size += size_t(len(arg) + 1)
	}
	return size
}

func (strs charbuffer) writeSizes(caller *wasmtime.Caller, _argc, _argv int32) error {
	argc := ptr_t(_argc)
	argv := ptr_t(_argv)
	return ensure(caller, func(base unsafe.Pointer, _ []byte) {
		*(*uint32)(unsafe.Add(base, argc)) = uint32(len(strs))
		*(*uint32)(unsafe.Add(base, argv)) = strs.size()
	}, argc+ptrSize, argv+ptrSize)
}

func (strs charbuffer) write(caller *wasmtime.Caller, _listptr, _bufptr int32) error {
	listptr := ptr_t(_listptr)
	bufptr := ptr_t(_bufptr)
	return ensure(caller, func(base unsafe.Pointer, data []byte) {
		var buf bytes.Buffer
		list0 := (*ptr_t)(unsafe.Add(base, listptr))
		list := unsafe.Slice(list0, len(strs))
		for i, arg := range strs {
			ptr := ptr_t(buf.Len()) + bufptr
			list[i] = ptr
			buf.WriteString(arg)
			buf.WriteByte(0)
		}
		copy(data[bufptr:], buf.Bytes())
	}, listptr+ptrSize*ptr_t(len(strs)), bufptr+strs.size())
}
