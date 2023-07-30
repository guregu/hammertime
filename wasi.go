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

func (wasi *WASI) args_sizes_get(caller *wasmtime.Caller, argc, argv libc.Int) (libc.Int, *wasmtime.Trap) {
	wasi.debugln("args_sizes_get", argc, argv)

	err := wasi.args.writeSizes(caller, argc, argv)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) environ_sizes_get(caller *wasmtime.Caller, argc, argv libc.Int) (libc.Int, *wasmtime.Trap) {
	wasi.debugln("environ_sizes_get", argc, argv)

	err := wasi.environ.writeSizes(caller, argc, argv)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) args_get(caller *wasmtime.Caller, argv, argbuf libc.Int) (libc.Int, *wasmtime.Trap) {
	wasi.debugln("args_get", argv, argbuf)

	if err := wasi.args.write(caller, argv, argbuf); err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) environ_get(caller *wasmtime.Caller, argv, argbuf libc.Int) (libc.Int, *wasmtime.Trap) {
	wasi.debugln("environ_get", argv, argbuf)

	err := wasi.environ.write(caller, argv, argbuf)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) fd_close(caller *wasmtime.Caller, fd libc.Int) (libc.Int, *wasmtime.Trap) {
	wasi.debugln("fd_close", fd)
	errno := wasi.close(fd)
	return errno, nil
}

func (wasi *WASI) fd_fdstat_get(caller *wasmtime.Caller, fd, _retptr libc.Int) (libc.Int, *wasmtime.Trap) {
	retptr := libc.Ptr(_retptr)
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
	}, retptr+libc.Size(unsafe.Sizeof(*stat)))

	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) fd_seek(caller *wasmtime.Caller, fd libc.Int, offset int64, whence, _retptr libc.Int) (libc.Int, *wasmtime.Trap) {
	retptr := libc.Ptr(_retptr)
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

func (wasi *WASI) fd_write(caller *wasmtime.Caller, fd, _iovs, _iovslen, _retptr libc.Int) (libc.Int, *wasmtime.Trap) {
	iovs := libc.Ptr(_iovs)
	iovslen := libc.Size(_iovslen)
	retptr := libc.Ptr(_retptr)
	wasi.debugln("fd_write", fd, iovs, iovslen, retptr)

	f, errno := wasi.get(fd)
	if errno != libc.ErrnoSuccess {
		return errno, nil
	}
	vecsize := libc.Size(unsafe.Sizeof(libc.Iovec{}))
	var total libc.Size
	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		vec0 := (*libc.Iovec)(unsafe.Add(base, iovs))
		vecs := unsafe.Slice(vec0, iovslen)
		for _, vec := range vecs {
			buf := data[vec.Buf : vec.Buf+vec.Len]
			wrote, err := f.Write(buf)
			total += libc.Size(wrote)
			wasi.debugf("write(%d, %q)", fd, string(buf))
			if err == io.EOF {
				break
			} else if err != nil {
				errno = libc.Error(err)
				break
			}
		}
		*(*libc.Size)(unsafe.Add(base, retptr)) = total
	}, iovs+vecsize*iovslen, retptr+libc.PtrSize)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	return errno, nil
}

func (wasi *WASI) proc_exit(caller *wasmtime.Caller, code libc.Int) *wasmtime.Trap {
	if code > 0 {
		return wasmtime.NewTrap(fmt.Sprintf("exit: %d", code))
	}
	return nil
}

func (wasi *WASI) clock_time_get(caller *wasmtime.Caller, clockid libc.Int, resolution int64, _tsptr libc.Int) (libc.Int, *wasmtime.Trap) {
	tsptr := libc.Ptr(_tsptr)
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

func (wasi *WASI) fd_fdstat_set_flags(caller *wasmtime.Caller, fd libc.Int, flags libc.Int) (libc.Int, *wasmtime.Trap) {
	wasi.debugf("fd_fdstat_set_flags(%d, %o)", fd, flags)
	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) fd_prestat_get(caller *wasmtime.Caller, fd libc.Int, _prestat libc.Int) (libc.Int, *wasmtime.Trap) {
	prestat := libc.Ptr(_prestat)
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
		DirLen: libc.Uint(len(f.preopen)),
	}

	err := ensure(caller, func(base unsafe.Pointer, _ []byte) {
		*(*libc.PrestatDir)(unsafe.Add(base, prestat)) = dir
	}, prestat+libc.Size(unsafe.Sizeof(dir)))
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) fd_prestat_dir_name(caller *wasmtime.Caller, fd int32, _buf int32, _len int32) (libc.Int, *wasmtime.Trap) {
	buf := libc.Ptr(_buf)
	len := libc.Size(_len)
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

func (wasi *WASI) fd_read(caller *wasmtime.Caller, fd, _iovs, _iovslen, _retptr int32) (libc.Int, *wasmtime.Trap) {
	iovs := libc.Ptr(_iovs)
	iovslen := libc.Size(_iovslen)
	retptr := libc.Ptr(_retptr)
	wasi.debugln("fd_read", fd, iovs, iovslen, retptr)

	f, errno := wasi.get(fd)
	if errno != libc.ErrnoSuccess {
		return errno, nil
	}

	vecsize := libc.Size(unsafe.Sizeof(libc.Ciovec{}))
	var total libc.Size
	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		vec0 := (*libc.Ciovec)(unsafe.Add(base, iovs))
		vecs := unsafe.Slice(vec0, iovslen)
		for _, vec := range vecs {
			buf := data[vec.Buf+total : vec.Buf+vec.Len]
			read, err := f.Read(buf)
			total += libc.Size(read)
			wasi.debugf("read(%d, %q, %d)", fd, string(buf[:read]), total)
			if err == io.EOF {
				break
			} else if err != nil {
				errno = libc.Error(err)
				break
			}
		}
		*(*libc.Size)(unsafe.Add(base, retptr)) = total
	}, iovs+vecsize*iovslen, retptr+libc.PtrSize)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	return errno, nil
}

func (wasi *WASI) fd_pread(caller *wasmtime.Caller, fd, _iovs, _iovslen, _offset, _retptr int32) (errno int32, trap *wasmtime.Trap) {
	iovs := libc.Ptr(_iovs)
	iovslen := libc.Size(_iovslen)
	offset := libc.Size(_offset)
	retptr := libc.Ptr(_retptr)
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

	vecsize := libc.Size(unsafe.Sizeof(libc.Iovec{}))
	var total libc.Size
	err = ensure(caller, func(base unsafe.Pointer, data []byte) {
		vec0 := (*libc.Iovec)(unsafe.Add(base, iovs))
		vecs := unsafe.Slice(vec0, iovslen)
		for _, vec := range vecs {
			buf := data[vec.Buf : vec.Buf+vec.Len]
			read, err := f.Read(buf)
			total += libc.Size(read)
			wasi.debugf("pread(%d, %q, %d)", fd, string(buf[:read]), total)
			if err == io.EOF {
				break
			} else if err != nil {
				errno = libc.Error(err)
				break
			}
		}
		*(*libc.Size)(unsafe.Add(base, retptr)) = total
	}, iovs+vecsize*iovslen, retptr+libc.PtrSize)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return errno, nil
}

func (wasi *WASI) fd_readdir(caller *wasmtime.Caller, fd, _buf, _buflen libc.Int, cookie int64, _retptr libc.Int) (libc.Int, *wasmtime.Trap) {
	buf := libc.Ptr(_buf)
	buflen := libc.Size(_buflen)
	retptr := libc.Ptr(_retptr) // buffer consumed
	wasi.debugln("fd_readdir", fd, buf, buflen, cookie, retptr)

	var errno libc.Errno
	size := libc.Size(unsafe.Sizeof(libc.Dirent{}))
	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		var wrote libc.Size
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
			n := libc.Size(copy(data[buf+wrote:buf+buflen], []byte(name)))
			wrote += n
			if n != libc.Size(len(name)) {
				break
			}
		}
		*(*libc.Size)(unsafe.Add(base, retptr)) = wrote
	}, buf+buflen, retptr+libc.PtrSize)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) path_open(caller *wasmtime.Caller, fd, dirflags, _pathptr, _pathlen, oflags int32, fs_rights_base, fs_rights_inheriting int64, fdflags, _retptr int32) (libc.Int, *wasmtime.Trap) {
	pathptr := libc.Ptr(_pathptr)
	pathlen := libc.Size(_pathlen)
	retptr := libc.Ptr(_retptr)
	wasi.debugln("path_open", fd, dirflags, pathptr, pathlen, oflags, fs_rights_base, fs_rights_inheriting, fdflags, retptr)
	var errno libc.Errno
	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		path := string(data[pathptr : pathptr+pathlen])
		wasi.debugf("path_open(%q, %o)", path, oflags)
		var file libc.Int
		file, errno = wasi.open(path)
		*(*libc.Int)(unsafe.Add(base, retptr)) = file
	}, pathptr+pathlen, retptr+libc.PtrSize)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	return errno, nil
}

func (wasi *WASI) path_create_directory(caller *wasmtime.Caller, _path, _pathlen int32) (libc.Int, *wasmtime.Trap) {
	path := libc.Ptr(_path)
	pathlen := libc.Size(_pathlen)

	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		name := string(data[path : path+pathlen])
		wasi.debugln("TODO: create dir", name)
	}, path+pathlen)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoNosys, nil
}

func (wasi *WASI) path_remove_directory(caller *wasmtime.Caller, _path, _pathlen int32) (libc.Int, *wasmtime.Trap) {
	path := libc.Ptr(_path)
	pathlen := libc.Size(_pathlen)

	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		name := string(data[path : path+pathlen])
		wasi.debugln("TODO: remove dir", name)
	}, path+pathlen)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoNosys, nil
}

func (wasi *WASI) path_unlink_file(caller *wasmtime.Caller, _path, _pathlen int32) (libc.Int, *wasmtime.Trap) {
	path := libc.Ptr(_path)
	pathlen := libc.Size(_pathlen)

	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		name := string(data[path : path+pathlen])
		wasi.debugln("TODO: unlink file", name)
	}, path+pathlen)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return libc.ErrnoNosys, nil
}

func (wasi *WASI) fd_filestat_get(caller *wasmtime.Caller, fd libc.Int, _retptr libc.Int) (libc.Int, *wasmtime.Trap) {
	retptr := libc.Ptr(_retptr)
	size := libc.Size(unsafe.Sizeof(libc.Filestat{}))

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

func (wasi *WASI) path_filestat_get(caller *wasmtime.Caller, fd, _lookupflags, _path, _pathlen, _retptr libc.Int) (libc.Int, *wasmtime.Trap) {
	flags := libc.Uint(_lookupflags)
	path := libc.Ptr(_path)
	pathlen := libc.Size(_pathlen)
	retptr := libc.Ptr(_retptr)
	wasi.debugln("path_filestat_get", fd, flags, path, pathlen, retptr)

	size := libc.Size(unsafe.Sizeof(libc.Filestat{}))

	stat, errno := wasi.stat(fd)
	if errno != 0 {
		return errno, nil
	}

	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		name := data[path : path+pathlen]
		wasi.debugln("name:", name)
		*(*libc.Filestat)(unsafe.Add(base, retptr)) = *stat
	}, path+pathlen, retptr+size)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	return libc.ErrnoSuccess, nil
}

func (wasi *WASI) path_readlink(caller *wasmtime.Caller, fd, _path, _pathlen, _bufptr, _buflen, _retptr libc.Int) (libc.Int, *wasmtime.Trap) {
	path := libc.Ptr(_path)
	pathlen := libc.Size(_pathlen)
	bufptr := libc.Ptr(_bufptr)
	buflen := libc.Size(_buflen)
	retptr := libc.Ptr(_retptr)
	wasi.debugln("path_readlink", path, pathlen, bufptr, buflen, retptr)

	var errno libc.Errno
	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		name := string(data[path : path+pathlen])
		var link string
		link, errno = wasi.readlink(name)
		if errno != 0 {
			return
		}
		size := libc.Size(copy(data[bufptr:bufptr+buflen], []byte(link)))
		*(*libc.Size)(unsafe.Add(base, retptr)) = size
	}, path+pathlen, bufptr+buflen)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return errno, nil
}

// __imported_wasi_snapshot_preview1_path_rename((int32_t) fd, (int32_t) old_path, (int32_t) old_path_len, (int32_t) new_fd, (int32_t) new_path, (int32_t) new_path_len)
func (wasi *WASI) path_rename(caller *wasmtime.Caller, fd, _oldpath, _oldpathlen, _newfdptr, _newpath, _newpathlen libc.Int) (libc.Int, *wasmtime.Trap) {
	oldpath := libc.Ptr(_oldpath)
	oldpathlen := libc.Size(_oldpathlen)
	newfd := libc.Ptr(_newfdptr)
	newpath := libc.Ptr(_newpath)
	newpathlen := libc.Size(_newpathlen)
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
		*(*libc.Int)(unsafe.Add(base, newfd)) = fd // TODO
	}, oldpath+oldpathlen, newpath+newpathlen, newfd)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return errno, nil
}

func (wasi *WASI) poll_oneoff(caller *wasmtime.Caller, _in, _out, _nsubs, _retptr libc.Int) (libc.Int, *wasmtime.Trap) {
	in := libc.Ptr(_in)
	out := libc.Ptr(_out)
	nsubs := libc.Size(_nsubs)
	retptr := libc.Ptr(_retptr)
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
