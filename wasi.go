package mywasi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"log"
	"runtime"
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go/v11"
)

type WASI struct {
	args    charbuffer
	environ charbuffer

	FDs    map[int32]FD
	nextfd int32
	fs     fs.FS
	clock  Clock

	// config
	stdout io.Writer
	stderr io.Writer
	env    map[string]string
}

type FD struct {
	io.Reader
	io.Writer
	stat *fdstat
}

func NewWASI(opts ...Option) *WASI {
	wasi := new(WASI)
	for _, opt := range opts {
		opt(wasi)
	}
	wasi.environ = make(charbuffer, len(wasi.env))
	for k, v := range wasi.env {
		wasi.environ = append(wasi.environ, fmt.Sprintf("%s=%s", k, v))
	}
	if wasi.clock == nil {
		wasi.clock = SystemClock
	}
	wasi.FDs = map[int32]FD{
		1: newFD(nil, wasi.stdout),
		2: newFD(nil, wasi.stderr),
	}
	if wasi.fs != nil {
		wasi.FDs[3] = FD{stat: &fdstat{fs_filetype: filetypeDirectory, fs_flags: 0}}
	}
	wasi.nextfd = 4
	return wasi
}

func (wasi *WASI) open(path string) (fd int32, errno Errno) {
	f, err := wasi.fs.Open(path)
	if err != nil {
		panic(err)
	}
	fd = wasi.nextfd
	wasi.nextfd++
	wasi.FDs[fd] = newFD(f, nil)
	return
}

func newFD(r io.Reader, w io.Writer) FD {
	if w == nil {
		w = io.Discard
	}
	return FD{
		Reader: r,
		Writer: w,
	}
}

func (wasi *WASI) getFD(fd int32) (FD, Errno) {
	f, ok := wasi.FDs[fd]
	if !ok {
		return FD{}, ErrnoBadf
	}
	return f, ErrnoSuccess
}

/*
  (import "wasi_snapshot_preview1" "args_get" (func $__imported_wasi_snapshot_preview1_args_get (type 5)))
  (import "wasi_snapshot_preview1" "args_sizes_get" (func $__imported_wasi_snapshot_preview1_args_sizes_get (type 5)))
  (import "wasi_snapshot_preview1" "environ_get" (func $__imported_wasi_snapshot_preview1_environ_get (type 5)))
  (import "wasi_snapshot_preview1" "environ_sizes_get" (func $__imported_wasi_snapshot_preview1_environ_sizes_get (type 5)))
  (import "wasi_snapshot_preview1" "clock_time_get" (func $__imported_wasi_snapshot_preview1_clock_time_get (type 7)))
  (import "wasi_snapshot_preview1" "fd_close" (func $__imported_wasi_snapshot_preview1_fd_close (type 0)))
  (import "wasi_snapshot_preview1" "fd_fdstat_get" (func $__imported_wasi_snapshot_preview1_fd_fdstat_get (type 5)))
  (import "wasi_snapshot_preview1" "fd_fdstat_set_flags" (func $__imported_wasi_snapshot_preview1_fd_fdstat_set_flags (type 5)))
  (import "wasi_snapshot_preview1" "fd_filestat_get" (func $__imported_wasi_snapshot_preview1_fd_filestat_get (type 5)))
  (import "wasi_snapshot_preview1" "fd_pread" (func $__imported_wasi_snapshot_preview1_fd_pread (type 8)))
  (import "wasi_snapshot_preview1" "fd_prestat_get" (func $__imported_wasi_snapshot_preview1_fd_prestat_get (type 5)))
  (import "wasi_snapshot_preview1" "fd_prestat_dir_name" (func $__imported_wasi_snapshot_preview1_fd_prestat_dir_name (type 3)))
  (import "wasi_snapshot_preview1" "fd_read" (func $__imported_wasi_snapshot_preview1_fd_read (type 2)))
  (import "wasi_snapshot_preview1" "fd_readdir" (func $__imported_wasi_snapshot_preview1_fd_readdir (type 8)))
  (import "wasi_snapshot_preview1" "fd_seek" (func $__imported_wasi_snapshot_preview1_fd_seek (type 9)))
  (import "wasi_snapshot_preview1" "fd_write" (func $__imported_wasi_snapshot_preview1_fd_write (type 2)))
  (import "wasi_snapshot_preview1" "path_create_directory" (func $__imported_wasi_snapshot_preview1_path_create_directory (type 3)))
  (import "wasi_snapshot_preview1" "path_filestat_get" (func $__imported_wasi_snapshot_preview1_path_filestat_get (type 6)))
  (import "wasi_snapshot_preview1" "path_open" (func $__imported_wasi_snapshot_preview1_path_open (type 10)))
  (import "wasi_snapshot_preview1" "path_readlink" (func $__imported_wasi_snapshot_preview1_path_readlink (type 11)))
  (import "wasi_snapshot_preview1" "path_remove_directory" (func $__imported_wasi_snapshot_preview1_path_remove_directory (type 3)))
  (import "wasi_snapshot_preview1" "path_rename" (func $__imported_wasi_snapshot_preview1_path_rename (type 11)))
  (import "wasi_snapshot_preview1" "path_unlink_file" (func $__imported_wasi_snapshot_preview1_path_unlink_file (type 3)))
  (import "wasi_snapshot_preview1" "poll_oneoff" (func $__imported_wasi_snapshot_preview1_poll_oneoff (type 2)))
  (import "wasi_snapshot_preview1" "proc_exit" (func $__imported_wasi_snapshot_preview1_proc_exit (type 12)))
*/

func (wasi *WASI) Define(store wasmtime.Storelike, linker *wasmtime.Linker) {
	const mod = "wasi_snapshot_preview1"
	// args_size_get := wasmtime.NewFunc(
	// 	store,
	// 	wasmtime.NewFuncType(
	// 		[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32), wasmtime.NewValType(wasmtime.KindI32)},
	// 		[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
	// 	),
	// 	wasi.args_sizes_get,
	// )
	linker.DefineFunc(store, mod, "args_sizes_get", wasi.args_sizes_get)
	linker.DefineFunc(store, mod, "args_get", wasi.args_get)
	linker.DefineFunc(store, mod, "environ_sizes_get", wasi.environ_sizes_get)
	linker.DefineFunc(store, mod, "environ_get", wasi.environ_get)
	linker.DefineFunc(store, mod, "clock_time_get", wasi.clock_time_get)
	linker.DefineFunc(store, mod, "fd_close", wasi.fd_close)
	linker.DefineFunc(store, mod, "fd_fdstat_get", wasi.fd_fdstat_get)
	linker.DefineFunc(store, mod, "fd_fdstat_set_flags", wasi.fd_fdstat_set_flags)
	linker.DefineFunc(store, mod, "fd_prestat_get", wasi.fd_prestat_get)
	linker.DefineFunc(store, mod, "fd_prestat_dir_name", wasi.fd_prestat_dir_name)
	linker.DefineFunc(store, mod, "fd_seek", wasi.fd_seek)
	linker.DefineFunc(store, mod, "fd_write", wasi.fd_write)
	linker.DefineFunc(store, mod, "fd_read", wasi.fd_read)
	linker.DefineFunc(store, mod, "path_open", wasi.path_open)
	linker.DefineFunc(store, mod, "proc_exit", wasi.proc_exit)
}

// func (wasi *WASI) args_sizes_get(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
// 	mem := caller.GetExport("memory").Memory()
// 	defer runtime.KeepAlive(mem)
// 	data := mem.UnsafeData(caller)
// 	binary.LittleEndian.PutUint32(data[uint32(args[0].I32()):], uint32(0))
// 	binary.LittleEndian.PutUint32(data[uint32(args[1].I32()):], uint32(0))
// 	return []wasmtime.Val{wasmtime.ValI32(0)}, nil
// }

func (wasi *WASI) args_sizes_get(caller *wasmtime.Caller, argc, argv int32) (int32, *wasmtime.Trap) {
	log.Println("args_sizes_get", argc, argv)

	err := wasi.args.writeSizes(caller, argc, argv)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return 0, nil
}

func (wasi *WASI) environ_sizes_get(caller *wasmtime.Caller, argc, argv int32) (int32, *wasmtime.Trap) {
	log.Println("environ_sizes_get", argc, argv)

	err := wasi.environ.writeSizes(caller, argc, argv)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return 0, nil
}

func (wasi *WASI) args_get(caller *wasmtime.Caller, argv, argbuf int32) (int32, *wasmtime.Trap) {
	log.Println("args_get", argv, argbuf)

	if err := wasi.args.write(caller, argv, argbuf); err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	return 0, nil
}

func (wasi *WASI) environ_get(caller *wasmtime.Caller, argv, argbuf int32) (int32, *wasmtime.Trap) {
	log.Println("environ_get", argv, argbuf)

	err := wasi.environ.write(caller, argv, argbuf)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return 0, nil
}

type charbuffer []string

func (strs charbuffer) size() uint32 {
	var size uint32
	for _, arg := range strs {
		size += uint32(len(arg) + 1)
	}
	return size
}

func (strs charbuffer) writeSizes(caller *wasmtime.Caller, argc, argv int32) error {
	return ensure(caller, func(p unsafe.Pointer) {
		*(*uint32)(unsafe.Add(p, uint32(argc))) = uint32(len(strs))
		*(*uint32)(unsafe.Add(p, uint32(argv))) = strs.size()
	}, argc+ptrSize, argv+ptrSize)
}

func (strs charbuffer) write(caller *wasmtime.Caller, listptr, bufptr int32) error {
	// TODO: safety
	mem := caller.GetExport("memory").Memory()
	defer runtime.KeepAlive(mem)
	data := mem.UnsafeData(caller)

	// return ensure(caller, func(p unsafe.Pointer) {

	// }, listptr, bufptr) // TODO: check max

	var buf bytes.Buffer
	for i, arg := range strs {
		ptr := uint32(buf.Len()) + uint32(bufptr)
		binary.LittleEndian.PutUint32(data[uint32(listptr)+uint32(i)*ptrSize:], ptr)
		buf.WriteString(arg)
		buf.WriteByte(0)
	}
	copy(data[uint32(bufptr):], buf.Bytes())

	return nil
}

func (wasi *WASI) fd_close(caller *wasmtime.Caller, fd int32) (int32, *wasmtime.Trap) {
	log.Println("fd_close", fd)
	return 0, nil
}

// func (wasi *WASI) fd_stat(caller *wasmtime.Caller, fd int32) (int32, *wasmtime.Trap) {
// 	return 0, nil
// }

func (wasi *WASI) fd_fdstat_get(caller *wasmtime.Caller, fd, retptr int32) (int32, *wasmtime.Trap) {
	log.Println("fd_fdstat_get", fd, retptr)

	var stat fdstat
	switch fd {
	case 0: // stdin
		stat = fdstat{fs_filetype: filetypeCharacterDevice, fs_flags: fdflagRSync}
	case 1: // stdout
		stat = fdstat{fs_filetype: filetypeCharacterDevice, fs_flags: fdflagAppend}
	case 2: // stderr
		stat = fdstat{fs_filetype: filetypeCharacterDevice, fs_flags: fdflagAppend}
	default:
		// stat = fdstat{fs_filetype: filetypeDirectory, fs_flags: 0}
		f, errno := wasi.getFD(fd)
		if errno != ErrnoSuccess {
			return errno, nil
		}
		switch {
		case f.stat != nil:
			stat = *f.stat
		case f.Writer != nil, f.Reader != nil:
			stat = fdstat{fs_filetype: filetypeRegularFile, fs_flags: 0}
		case f.Reader != nil:
			return ErrnoBadf, nil
		}
	}

	ensure(caller, func(p unsafe.Pointer) {
		*(*fdstat)(unsafe.Add(p, retptr)) = stat
	}, retptr+int32(unsafe.Sizeof(stat)))

	return 0, nil
}

// (param i32 i64 i32 i32) (result i32)))
func (wasi *WASI) fd_seek(caller *wasmtime.Caller, a int32, b int64, c, d int32) (int32, *wasmtime.Trap) {
	log.Println("fd_seek", a, b, c, d)
	return 0, nil
}

func (wasi *WASI) fd_write(caller *wasmtime.Caller, fd, iovs, iovslen, retptr int32) (int32, *wasmtime.Trap) {
	log.Println("fd_write", fd, iovs, iovslen, retptr)

	mem := caller.GetExport("memory").Memory()
	defer runtime.KeepAlive(mem)
	data := mem.UnsafeData(caller)
	datap := mem.Data(caller)
	memsize := mem.DataSize(caller)

	f, errno := wasi.getFD(fd)
	if errno != ErrnoSuccess {
		return errno, nil
	}

	var total int
	for i := int32(0); i < iovslen; i++ {
		if iovs+(i+1)*8 > int32(memsize) {
			return 0, wasmtime.NewTrap(fmt.Sprintf("oob: %d", iovs+i*8))
		}
		// TODO: bounds checking lol
		vec := (*ciovec)(unsafe.Add(datap, iovs+i*8))
		// log.Println(unsafe.Sizeof(ciovec_t{}))
		if vec.buf == 0 || vec.buf_len == 0 {
			continue
		}
		wrote, err := f.Write(data[vec.buf : vec.buf+vec.buf_len])
		if err != nil {
			// handle err
			panic(err)
		}
		total += wrote
	}
	*(*uint32)(unsafe.Add(datap, retptr)) = uint32(total)
	return 0, nil
}

func (wasi *WASI) proc_exit(caller *wasmtime.Caller, code int32) *wasmtime.Trap {
	log.Println("proc_exit", code)
	return nil
}

func (wasi *WASI) clock_time_get(caller *wasmtime.Caller, clockid int32, resolution int64, tsptr int32) (int32, *wasmtime.Trap) {
	log.Println("clock_time_get", clockid, resolution, tsptr)
	err := ensure(caller, func(p unsafe.Pointer) {
		*(*int64)(unsafe.Add(p, tsptr)) = wasi.clock.Now().UnixNano()
	}, tsptr+ptrSize)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	return 0, nil
}

func (wasi *WASI) fd_fdstat_set_flags(caller *wasmtime.Caller, fd int32, flags int32) (int32, *wasmtime.Trap) {
	log.Println("fd_fdstat_set_flags", fd, flags)
	return 0, nil
}

func (wasi *WASI) fd_prestat_get(caller *wasmtime.Caller, fd int32, prestat int32) (int32, *wasmtime.Trap) {
	log.Println("fd_prestat_get", fd, prestat)

	if fd > 3 {
		return ErrnoBadf, nil
	}

	dir := prestat_dir{
		tag:     0, // directory
		dir_len: 1, // len("/")
	}

	err := ensure(caller, func(p unsafe.Pointer) {
		*(*prestat_dir)(unsafe.Add(p, prestat)) = dir
	}, prestat+int32(unsafe.Sizeof(dir)))
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return 0, nil
}

func (wasi *WASI) fd_prestat_dir_name(caller *wasmtime.Caller, fd int32, buf int32, len int32) (int32, *wasmtime.Trap) {
	log.Println("fd_prestat_dir_name", fd, buf, len)

	if fd != 3 {
		return ErrnoBadf, nil
	}

	name := "/"

	mem := caller.GetExport("memory").Memory()
	defer runtime.KeepAlive(mem)
	data := mem.UnsafeData(caller)
	copy(data[uint32(buf):uint32(buf+len)], []byte(name))

	return 0, nil
}

func (wasi *WASI) fd_read(caller *wasmtime.Caller, fd, iovs, iovslen, retptr int32) (int32, *wasmtime.Trap) {
	log.Println("fd_read", fd, iovs, iovslen, retptr)

	f, errno := wasi.getFD(fd)
	if errno != ErrnoSuccess {
		return errno, nil
	}

	vecsize := int32(unsafe.Sizeof(iovec{}))
	var total size_t
	err := ensure2(caller, func(base unsafe.Pointer, data []byte) {
		vec0 := (*iovec)(unsafe.Add(base, iovs))
		vecs := unsafe.Slice(vec0, iovslen)
		for _, vec := range vecs {
			buf := data[vec.buf : vec.buf+vec.buf_len]
			read, err := f.Reader.Read(buf)
			total += size_t(read)
			if err == io.EOF {
				// errno = -1
			} else if err != nil {
				// TODO
				panic(err)
			}
		}
		*(*size_t)(unsafe.Add(base, retptr)) = total
	}, iovs+vecsize*iovslen, retptr+ptrSize)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	return errno, nil
}

func (wasi *WASI) path_open(caller *wasmtime.Caller, fd, dirflags, pathptr, pathlen, oflags int32, fs_rights_base, fs_rights_inheriting int64, fdflags, retptr int32) (int32, *wasmtime.Trap) {
	log.Println("path_open", fd, dirflags, pathptr, pathlen, oflags, fs_rights_base, fs_rights_inheriting, fdflags, retptr)
	var errno Errno
	ensure2(caller, func(base unsafe.Pointer, data []byte) {
		path := string(data[pathptr : pathptr+pathlen])
		var file int32
		file, errno = wasi.open(path)
		*(*int32)(unsafe.Add(base, retptr)) = file
	}, pathptr+pathlen, retptr+ptrSize)
	return errno, nil
}

/*
__wasi_errno_t __wasi_path_open(
    __wasi_fd_t fd,
    __wasi_lookupflags_t dirflags,
    const char *path,
    __wasi_oflags_t oflags,
    __wasi_rights_t fs_rights_base,
    __wasi_rights_t fs_rights_inheriting,
    __wasi_fdflags_t fdflags,
    __wasi_fd_t *retptr0
){
    size_t path_len = strlen(path);
    int32_t ret = __imported_wasi_snapshot_preview1_path_open((int32_t) fd, dirflags, (int32_t) path, (int32_t) path_len, oflags, fs_rights_base, fs_rights_inheriting, fdflags, (int32_t) retptr0);
    return (uint16_t) ret;
}*/

const ptrSize = 4
