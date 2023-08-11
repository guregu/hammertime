package hammertime

import (
	"fmt"
	"io"
	"log"
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go/v11"
	"github.com/hack-pad/hackpadfs"

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
	wasi.debugf("seek(%d, %d, %d)", fd, offset, whence)
	f, errno := wasi.get(fd)
	if errno != libc.ErrnoSuccess {
		return errno, nil
	}
	ret, err := hackpadfs.SeekFile(f.File, offset, int(whence))
	if err != nil {
		// TODO: handle EPIPE?
		return libc.Error(err), nil
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
	return libc.ErrnoNosys, nil
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

func (wasi *WASI) fd_prestat_dir_name(caller *wasmtime.Caller, fd, _buf, _len libc.Int) (libc.Int, *wasmtime.Trap) {
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

func (wasi *WASI) fd_read(caller *wasmtime.Caller, fd, _iovs, _iovslen, _retptr libc.Int) (libc.Int, *wasmtime.Trap) {
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

func (wasi *WASI) fd_pread(caller *wasmtime.Caller, fd, _iovs, _iovslen libc.Int, _offset int64, _retptr libc.Int) (errno int32, trap *wasmtime.Trap) {
	iovs := libc.Ptr(_iovs)
	iovslen := libc.Size(_iovslen)
	offset := uint64(_offset)
	retptr := libc.Ptr(_retptr)
	wasi.debugln("fd_read", fd, iovs, iovslen, retptr)

	var f *filedesc
	f, errno = wasi.get(fd)
	if errno != libc.ErrnoSuccess {
		return
	}

	// TODO: use ReadAt

	pos, err := hackpadfs.SeekFile(f.File, 0, io.SeekCurrent)
	if err != nil {
		errno = libc.Error(err)
		return
	}
	defer func() {
		_, err := hackpadfs.SeekFile(f.File, pos, io.SeekStart)
		if errno == 0 {
			errno = libc.Error(err)
		}
	}()
	if pos != 0 && offset != 0 {
		_, err = hackpadfs.SeekFile(f.File, int64(offset), io.SeekStart)
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

func (wasi *WASI) path_open(caller *wasmtime.Caller, fd, _dirflags, _pathptr, _pathlen, _oflags int32, _fsrights_base, _fsrights_inheriting int64, _fdflags, _retptr int32) (libc.Int, *wasmtime.Trap) {
	dirflags := libc.Lookupflag(_dirflags)
	pathptr := libc.Ptr(_pathptr)
	pathlen := libc.Size(_pathlen)
	oflags := libc.Oflag(_oflags)
	fdflags := libc.Fdflag(_fdflags)
	rights := libc.Rights(_fsrights_base)
	retptr := libc.Ptr(_retptr)
	wasi.debugln("path_open", fd, dirflags, pathptr, pathlen, oflags, rights, fdflags, retptr)

	var errno libc.Errno
	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		path := string(data[pathptr : pathptr+pathlen])
		var file libc.Int
		file, errno = wasi.open(fd, path, dirflags, oflags, fdflags, rights)
		*(*libc.Int)(unsafe.Add(base, retptr)) = file
		wasi.debugf("open(%d, %q, %o, %o, %o) → %d", fd, path, oflags, fdflags, rights, errno)
	}, pathptr+pathlen, retptr+libc.PtrSize)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	return errno, nil
}

func (wasi *WASI) path_create_directory(caller *wasmtime.Caller, fd, _path, _pathlen int32) (libc.Int, *wasmtime.Trap) {
	path := libc.Ptr(_path)
	pathlen := libc.Size(_pathlen)

	var errno libc.Errno
	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		name := string(data[path : path+pathlen])
		errno = wasi.mkdir(fd, name, mkdirMode) // TODO: mkdirat
		wasi.debugf("mkdir(%d, %q)", fd, name)
	}, path+pathlen)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return errno, nil
}

func (wasi *WASI) path_remove_directory(caller *wasmtime.Caller, fd, _path, _pathlen int32) (libc.Int, *wasmtime.Trap) {
	path := libc.Ptr(_path)
	pathlen := libc.Size(_pathlen)

	var errno libc.Errno
	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		name := string(data[path : path+pathlen])
		errno = wasi.remove(fd, name)
		wasi.debugf("rmdir(%d, %q)", fd, name) // TODO: fd
	}, path+pathlen)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return errno, nil
}

func (wasi *WASI) path_unlink_file(caller *wasmtime.Caller, fd int32, _path, _pathlen int32) (libc.Int, *wasmtime.Trap) {
	path := libc.Ptr(_path)
	pathlen := libc.Size(_pathlen)

	var errno libc.Errno
	err := ensure(caller, func(base unsafe.Pointer, data []byte) {
		name := string(data[path : path+pathlen])
		errno = wasi.remove(fd, name)
		wasi.debugf("unlink(%d, %q)", fd, name)
	}, path+pathlen)
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}

	return errno, nil
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
		wasi.debugf("stat(%d, %q, %o)", fd, name, flags)
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
		link, errno = wasi.readlink(fd, name)
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
		oldname := string(data[oldpath : oldpath+oldpathlen])
		newname := string(data[newpath : newpath+newpathlen])
		errno = wasi.rename(fd, oldname, newname)
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
