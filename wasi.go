package hammertime

import (
	"fmt"
	"io"
	"log"
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go/v11"

	"github.com/guregu/hammertime/libc"
)

// WASI is a WASI environment.
type WASI struct {
	args    charbuffer
	environ charbuffer
	filesystem
	clock Clock

	// config
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	env    map[string]string
	debug  bool
}

// NewWASI creates a new WASI environment.
// Currently they may not be shared between instances.
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
	wasi.filesystem = *newFilesystem(wasi.fs, wasi.stdin, wasi.stdout, wasi.stderr)
	return wasi
}

// Link defines all (supported) WASI functions on the given linker.
func (wasi *WASI) Link(store wasmtime.Storelike, linker *wasmtime.Linker) error {
	const mod = "wasi_snapshot_preview1"
	symbols := map[string]any{
		"args_sizes_get":        wasi.args_sizes_get,
		"args_get":              wasi.args_get,
		"environ_sizes_get":     wasi.environ_sizes_get,
		"environ_get":           wasi.environ_get,
		"clock_time_get":        wasi.clock_time_get,
		"fd_close":              wasi.fd_close,
		"fd_fdstat_get":         wasi.fd_fdstat_get,
		"fd_fdstat_set_flags":   wasi.fd_fdstat_set_flags,
		"fd_prestat_get":        wasi.fd_prestat_get,
		"fd_prestat_dir_name":   wasi.fd_prestat_dir_name,
		"fd_filestat_get":       wasi.fd_filestat_get,
		"fd_seek":               wasi.fd_seek,
		"fd_write":              wasi.fd_write,
		"fd_read":               wasi.fd_read,
		"fd_pread":              wasi.fd_pread,
		"fd_readdir":            wasi.fd_readdir,
		"path_open":             wasi.path_open,
		"path_filestat_get":     wasi.path_filestat_get,
		"path_readlink":         wasi.path_readlink,
		"path_rename":           wasi.path_rename,
		"path_create_directory": wasi.path_create_directory,
		"path_remove_directory": wasi.path_remove_directory,
		"path_unlink_file":      wasi.path_unlink_file,
		"poll_oneoff":           wasi.poll_oneoff,
		"proc_exit":             wasi.proc_exit,
	}
	for name, fn := range symbols {
		if err := linker.DefineFunc(store, mod, name, fn); err != nil {
			return err
		}
	}
	return nil
}

func (wasi *WASI) args_sizes_get(caller *wasmtime.Caller, argc, argv int32) (int32, *wasmtime.Trap) {
	wasi.debugln("args_sizes_get", argc, argv)

	err := wasi.args.writeSizes(caller, argc, argv)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) environ_sizes_get(caller *wasmtime.Caller, argc, argv int32) (int32, *wasmtime.Trap) {
	wasi.debugln("environ_sizes_get", argc, argv)

	err := wasi.environ.writeSizes(caller, argc, argv)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) args_get(caller *wasmtime.Caller, argv, argbuf int32) (int32, *wasmtime.Trap) {
	wasi.debugln("args_get", argv, argbuf)

	if err := wasi.args.write(caller, argv, argbuf); err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) environ_get(caller *wasmtime.Caller, argv, argbuf int32) (int32, *wasmtime.Trap) {
	wasi.debugln("environ_get", argv, argbuf)

	err := wasi.environ.write(caller, argv, argbuf)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) fd_close(caller *wasmtime.Caller, fd int32) (int32, *wasmtime.Trap) {
	wasi.debugln("fd_close", fd)
	errno := wasi.close(fd)
	return errno, nil
}

func (wasi *WASI) fd_fdstat_get(caller *wasmtime.Caller, fd, _retptr int32) (int32, *wasmtime.Trap) {
	retptr := ptr_t(_retptr)
	wasi.debugln("fd_fdstat_get", fd, retptr)

	stat, errno := wasi.fdstat(fd)
	if errno != libc.ErrnoSuccess {
		return errno, nil
	}
	if stat == nil {
		stat = &libc.Fdstat{Filetype: libc.FiletypeUnknown}
	}

	ensure(caller, func(base unsafe.Pointer, _ []byte) {
		*(*libc.Fdstat)(unsafe.Add(base, retptr)) = *stat
	}, retptr+size_t(unsafe.Sizeof(*stat)))

	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) fd_seek(caller *wasmtime.Caller, fd int_t, offset int64, whence, _retptr int_t) (int32, *wasmtime.Trap) {
	retptr := ptr_t(_retptr)
	wasi.debugln("fd_seek", fd, offset, whence, retptr)
	f, errno := wasi.get(fd)
	if errno != libc.ErrnoSuccess {
		return errno, nil
	}
	ret, err := f.Seek(offset, int(whence))
	switch {
	// TODO: more
	case err != nil:
		wasi.debugln("seek error", err)
		return libc.ErrnoInval, nil
	}

	err = ensure(caller, func(base unsafe.Pointer, _ []byte) {
		*(*int64)(unsafe.Add(base, retptr)) = ret
	}, retptr+8)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) fd_write(caller *wasmtime.Caller, fd, _iovs, _iovslen, _retptr int32) (int32, *wasmtime.Trap) {
	iovs := ptr_t(_iovs)
	iovslen := size_t(_iovslen)
	retptr := ptr_t(_retptr)
	wasi.debugln("fd_write", fd, iovs, iovslen, retptr)

	f, errno := wasi.get(fd)
	if errno != libc.ErrnoSuccess {
		return errno, nil
	}
	vecsize := size_t(unsafe.Sizeof(libc.Iovec{}))
	var total size_t
	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		vec0 := (*libc.Iovec)(unsafe.Add(base, iovs))
		vecs := unsafe.Slice(vec0, iovslen)
		for _, vec := range vecs {
			buf := data[vec.Buf : vec.Buf+vec.Len]
			wrote, err := f.Write(buf)
			total += size_t(wrote)
			wasi.debugf("write(%d, %q)", fd, string(buf))
			if err == io.EOF {
				break
			} else if err != nil {
				errno = libc.Error(err)
				break
			}
		}
		*(*size_t)(unsafe.Add(base, retptr)) = total
	}, iovs+vecsize*iovslen, retptr+ptrSize)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	return errno, nil
}

func (wasi *WASI) proc_exit(caller *wasmtime.Caller, code int32) *wasmtime.Trap {
	if code > 0 {
		return wasmtime.NewTrap(fmt.Sprintf("exit: %d", code))
	}
	return nil
}

func (wasi *WASI) clock_time_get(caller *wasmtime.Caller, clockid int32, resolution int64, _tsptr int32) (int32, *wasmtime.Trap) {
	tsptr := ptr_t(_tsptr)
	wasi.debugln("clock_time_get", clockid, resolution, tsptr)

	// TODO: clockids

	err := ensure(caller, func(base unsafe.Pointer, _ []byte) {
		*(*int64)(unsafe.Add(base, tsptr)) = wasi.clock.Now().UnixNano()
	}, tsptr+8)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) fd_fdstat_set_flags(caller *wasmtime.Caller, fd int32, flags int32) (int32, *wasmtime.Trap) {
	wasi.debugf("fd_fdstat_set_flags(%d, %o)", fd, flags)
	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) fd_prestat_get(caller *wasmtime.Caller, fd int32, _prestat int32) (int32, *wasmtime.Trap) {
	prestat := ptr_t(_prestat)
	wasi.debugln("fd_prestat_get", fd, prestat)

	f, errno := wasi.get(fd)
	if errno != libc.ErrnoSuccess {
		return errno, nil
	}
	if f.preopen == "" {
		return libc.ErrnoBadf, nil
	}

	dir := libc.PrestatDir{
		Tag:    0, // directory
		DirLen: uint32(len(f.preopen)),
	}

	err := ensure(caller, func(base unsafe.Pointer, _ []byte) {
		*(*libc.PrestatDir)(unsafe.Add(base, prestat)) = dir
	}, prestat+size_t(unsafe.Sizeof(dir)))
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) fd_prestat_dir_name(caller *wasmtime.Caller, fd int32, _buf int32, _len int32) (int32, *wasmtime.Trap) {
	buf := ptr_t(_buf)
	len := size_t(_len)
	wasi.debugln("fd_prestat_dir_name", fd, buf, len)

	f, errno := wasi.get(fd)
	if errno != libc.ErrnoSuccess {
		return errno, nil
	}
	if f.preopen == "" {
		return libc.ErrnoBadf, nil
	}

	err := ensure(caller, func(_ unsafe.Pointer, data []byte) {
		copy(data[buf:buf+len], []byte(f.preopen))
	}, buf+len)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) fd_read(caller *wasmtime.Caller, fd, _iovs, _iovslen, _retptr int32) (int32, *wasmtime.Trap) {
	iovs := ptr_t(_iovs)
	iovslen := size_t(_iovslen)
	retptr := ptr_t(_retptr)
	wasi.debugln("fd_read", fd, iovs, iovslen, retptr)

	f, errno := wasi.get(fd)
	if errno != libc.ErrnoSuccess {
		return errno, nil
	}

	vecsize := size_t(unsafe.Sizeof(libc.Ciovec{}))
	var total size_t
	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		vec0 := (*libc.Ciovec)(unsafe.Add(base, iovs))
		vecs := unsafe.Slice(vec0, iovslen)
		for _, vec := range vecs {
			buf := data[vec.Buf+total : vec.Buf+vec.Len]
			read, err := f.Read(buf)
			total += size_t(read)
			wasi.debugf("read(%d, %q, %d)", fd, string(buf[:read]), total)
			if err == io.EOF {
				break
			} else if err != nil {
				errno = libc.Error(err)
				break
			}
		}
		*(*size_t)(unsafe.Add(base, retptr)) = total
	}, iovs+vecsize*iovslen, retptr+ptrSize)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	return errno, nil
}

func (wasi *WASI) fd_pread(caller *wasmtime.Caller, fd, _iovs, _iovslen, _offset, _retptr int32) (errno int32, trap *wasmtime.Trap) {
	iovs := ptr_t(_iovs)
	iovslen := size_t(_iovslen)
	offset := size_t(_offset)
	retptr := ptr_t(_retptr)
	wasi.debugln("fd_read", fd, iovs, iovslen, retptr)

	var f *filedesc
	f, errno = wasi.get(fd)
	if errno != libc.ErrnoSuccess {
		return
	}

	pos, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		errno = libc.Error(err)
		return
	}
	defer func() {
		_, err := f.Seek(pos, io.SeekStart)
		if errno == 0 {
			errno = libc.Error(err)
		}
	}()
	if pos != 0 && offset != 0 {
		_, err = f.Seek(int64(offset), io.SeekStart)
	}
	if err != nil {
		errno = libc.Error(err)
		return
	}

	vecsize := size_t(unsafe.Sizeof(libc.Iovec{}))
	var total size_t
	err = ensure(caller, func(base unsafe.Pointer, data []byte) {
		vec0 := (*libc.Iovec)(unsafe.Add(base, iovs))
		vecs := unsafe.Slice(vec0, iovslen)
		for _, vec := range vecs {
			buf := data[vec.Buf : vec.Buf+vec.Len]
			read, err := f.Read(buf)
			total += size_t(read)
			wasi.debugf("pread(%d, %q, %d)", fd, string(buf[:read]), total)
			if err == io.EOF {
				break
			} else if err != nil {
				errno = libc.Error(err)
				break
			}
		}
		*(*size_t)(unsafe.Add(base, retptr)) = total
	}, iovs+vecsize*iovslen, retptr+ptrSize)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return errno, nil
}

func (wasi *WASI) fd_readdir(caller *wasmtime.Caller, fd, _buf, _buflen int_t, cookie int64, _retptr int_t) (int_t, *wasmtime.Trap) {
	buf := ptr_t(_buf)
	buflen := size_t(_buflen)
	retptr := ptr_t(_retptr) // buffer consumed
	wasi.debugln("fd_readdir", fd, buf, buflen, cookie, retptr)

	var errno libc.Errno
	size := size_t(unsafe.Sizeof(libc.Dirent{}))
	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		var wrote size_t
		var dirp *libc.Dirent
		var name string
		for ; ; cookie++ {
			dirp, name, errno = wasi.readdir(fd, cookie)
			if errno != 0 || dirp == nil || name == "" {
				break
			}
			if wrote+size > buflen {
				break
			}
			*(*libc.Dirent)(unsafe.Add(base, buf+wrote)) = *dirp
			wrote += size
			n := size_t(copy(data[buf+wrote:buf+buflen], []byte(name)))
			wrote += n
			if n != size_t(len(name)) {
				break
			}
		}
		*(*size_t)(unsafe.Add(base, retptr)) = wrote
	}, buf+buflen, retptr+ptrSize)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) path_open(caller *wasmtime.Caller, fd, dirflags, _pathptr, _pathlen, oflags int32, fs_rights_base, fs_rights_inheriting int64, fdflags, _retptr int32) (int32, *wasmtime.Trap) {
	pathptr := ptr_t(_pathptr)
	pathlen := size_t(_pathlen)
	retptr := ptr_t(_retptr)
	wasi.debugln("path_open", fd, dirflags, pathptr, pathlen, oflags, fs_rights_base, fs_rights_inheriting, fdflags, retptr)
	var errno libc.Errno
	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		path := string(data[pathptr : pathptr+pathlen])
		wasi.debugf("path_open(%q, %o)", path, oflags)
		var file int_t
		file, errno = wasi.open(path)
		*(*int_t)(unsafe.Add(base, retptr)) = file
	}, pathptr+pathlen, retptr+ptrSize)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	return errno, nil
}

func (wasi *WASI) path_create_directory(caller *wasmtime.Caller, _path, _pathlen int32) (int32, *wasmtime.Trap) {
	path := ptr_t(_path)
	pathlen := size_t(_pathlen)

	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		name := string(data[path : path+pathlen])
		wasi.debugln("TODO: create dir", name)
	}, path+pathlen)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoNosys, nil
}

func (wasi *WASI) path_remove_directory(caller *wasmtime.Caller, _path, _pathlen int32) (int32, *wasmtime.Trap) {
	path := ptr_t(_path)
	pathlen := size_t(_pathlen)

	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		name := string(data[path : path+pathlen])
		wasi.debugln("TODO: remove dir", name)
	}, path+pathlen)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoNosys, nil
}

func (wasi *WASI) path_unlink_file(caller *wasmtime.Caller, _path, _pathlen int32) (int32, *wasmtime.Trap) {
	path := ptr_t(_path)
	pathlen := size_t(_pathlen)

	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		name := string(data[path : path+pathlen])
		wasi.debugln("TODO: unlink file", name)
	}, path+pathlen)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoNosys, nil
}

func (wasi *WASI) fd_filestat_get(caller *wasmtime.Caller, fd int_t, _retptr int_t) (int32, *wasmtime.Trap) {
	retptr := ptr_t(_retptr)
	size := size_t(unsafe.Sizeof(libc.Filestat{}))

	stat, errno := wasi.stat(fd)
	if errno != 0 {
		return errno, nil
	}

	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		*(*libc.Filestat)(unsafe.Add(base, retptr)) = *stat
	}, retptr+size)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) path_filestat_get(caller *wasmtime.Caller, fd, _lookupflags, _path, _pathlen, _retptr int_t) (int32, *wasmtime.Trap) {
	flags := uint_t(_lookupflags)
	path := ptr_t(_path)
	pathlen := size_t(_pathlen)
	retptr := ptr_t(_retptr)
	wasi.debugln("path_filestat_get", fd, flags, path, pathlen, retptr)

	size := size_t(unsafe.Sizeof(libc.Filestat{}))

	stat, errno := wasi.stat(fd)
	if errno != 0 {
		return errno, nil
	}

	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		name := data[path : path+pathlen]
		wasi.debugln("name:", name)
		*(*libc.Filestat)(unsafe.Add(base, retptr)) = *stat
	}, retptr+size)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) path_readlink(caller *wasmtime.Caller, fd, _path, _pathlen, _bufptr, _buflen, _retptr int_t) (int32, *wasmtime.Trap) {
	path := ptr_t(_path)
	pathlen := size_t(_pathlen)
	bufptr := ptr_t(_bufptr)
	buflen := size_t(_buflen)
	retptr := ptr_t(_retptr)
	wasi.debugln("path_readlink", path, pathlen, bufptr, buflen, retptr)

	var errno libc.Errno
	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		name := string(data[path : path+pathlen])
		var link string
		link, errno = wasi.readlink(name)
		if errno != 0 {
			return
		}
		size := size_t(copy(data[bufptr:bufptr+buflen], []byte(link)))
		*(*size_t)(unsafe.Add(base, retptr)) = size
	}, path+pathlen, bufptr+buflen)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return errno, nil
}

// __imported_wasi_snapshot_preview1_path_rename((int32_t) fd, (int32_t) old_path, (int32_t) old_path_len, (int32_t) new_fd, (int32_t) new_path, (int32_t) new_path_len)
func (wasi *WASI) path_rename(caller *wasmtime.Caller, fd, _oldpath, _oldpathlen, _newfdptr, _newpath, _newpathlen int_t) (int32, *wasmtime.Trap) {
	oldpath := ptr_t(_oldpath)
	oldpathlen := size_t(_oldpathlen)
	newfd := ptr_t(_newfdptr)
	newpath := ptr_t(_newpath)
	newpathlen := size_t(_newpathlen)
	// retptr := ptr_t(_retptr)
	wasi.debugln("path_rename", oldpath, oldpathlen, newfd, newpath, newpathlen)

	var errno libc.Errno
	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		// TODO: impl
		oldname := string(data[oldpath : oldpath+oldpathlen])
		newname := string(data[newpath : newpath+newpathlen])
		errno = wasi.rename(oldname, newname)
		if errno != 0 {
			return
		}
		*(*int_t)(unsafe.Add(base, newfd)) = fd // TODO
	}, oldpath+oldpathlen, newpath+newpathlen, newfd)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return errno, nil
}

func (wasi *WASI) poll_oneoff(caller *wasmtime.Caller, _in, _out, _nsubs, _retptr int_t) (int32, *wasmtime.Trap) {
	in := ptr_t(_in)
	out := ptr_t(_out)
	nsubs := size_t(_nsubs)
	retptr := ptr_t(_retptr)
	wasi.debugln("poll_oneoff", in, out, nsubs, retptr)
	return libc.ErrnoNosys, nil
}

func (wasi *WASI) debugln(args ...any) {
	if !wasi.debug {
		return
	}
	log.Println(args...)
}

func (wasi *WASI) debugf(fmt string, args ...any) {
	if !wasi.debug {
		return
	}
	log.Printf(fmt, args...)
}
