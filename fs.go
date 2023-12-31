package hammertime

import (
	"hash/crc64"
	"io"
	"io/fs"
	"path"
	"strconv"
	"syscall"
	"time"

	"github.com/hack-pad/hackpadfs"

	"github.com/guregu/hammertime/libc"
)

const (
	stdioMaxFD = 3
	rootFD     = 3
	mkdirMode  = 0755
)

type filesystem struct {
	fds    map[libc.Int]*filedesc
	nextfd libc.Int
	fs     hackpadfs.FS
	dev    uint64
}

func newFilesystem(fsys fs.FS, stdin io.Reader, stdout, stderr io.Writer) *filesystem {
	system := &filesystem{
		fds:    map[int32]*filedesc{},
		fs:     fsys,
		nextfd: stdioMaxFD + 1,
	}
	fd0 := newStream(stdin)
	fd1 := newStream(stdout)
	fd2 := newStream(stderr)
	system.set(0, fd0)
	system.set(1, fd1)
	system.set(2, fd2)
	if fsys != nil {
		fd3 := &filedesc{
			no:      3,
			fdstat:  &libc.Fdstat{Filetype: libc.FiletypeDirectory},
			preopen: "/",
		}
		system.set(3, fd3)
	}

	return system
}

func (fsys *filesystem) set(no libc.Int, fd *filedesc) {
	fd.no = no
	fsys.fds[no] = fd
	if fsys.nextfd <= no {
		fsys.nextfd = no + 1
	}
}

func (fsys *filesystem) get(fd libc.Int) (*filedesc, libc.Errno) {
	f, ok := fsys.fds[fd]
	if !ok {
		return nil, libc.ErrnoBadf
	}
	return f, libc.ErrnoSuccess
}

var crctab = crc64.MakeTable(crc64.ECMA)

func (fsys *filesystem) ino(fd libc.Int, name string) uint64 {
	// TODO: should use abs path
	if name == "" {
		name = "/proc/fd/" + strconv.Itoa(int(fd))
	}
	return crc64.Checksum([]byte(name), crctab)
}

func (fsys *filesystem) fdstat(fd libc.Int) (*libc.Fdstat, libc.Errno) {
	f, errno := fsys.get(fd)
	if errno != libc.ErrnoSuccess {
		return nil, errno
	}
	return f.fdstat, libc.ErrnoSuccess
}

func (fsys *filesystem) stat(fd libc.Int) (*libc.Filestat, libc.Errno) {
	f, errno := fsys.get(fd)
	if errno != libc.ErrnoSuccess {
		return nil, errno
	}
	stat, err := f.Stat()
	if err != nil {
		return nil, libc.Error(err)
	}

	fstat := &libc.Filestat{
		Dev:   fsys.dev,
		Ino:   fsys.ino(fd, stat.Name()),
		Mtim:  uint64(stat.ModTime().UnixNano()),
		Nlink: 1,
		Size:  uint64(stat.Size()),
	}
	if stat.IsDir() {
		fstat.Filetype = libc.FiletypeDirectory
	} else {
		fstat.Filetype = libc.FiletypeRegularFile // TODO
	}
	return fstat, libc.ErrnoSuccess
}

func (fsys *filesystem) open(basefd libc.Int, path string, dirflags libc.Lookupflag, oflags libc.Oflag, fdflags libc.Fdflag, rights libc.Rights) (libc.Int, libc.Errno) {
	if fsys.fs == nil {
		return 0, libc.ErrnoNosys
	}
	path, errno := fsys.rel(basefd, path)
	if errno != libc.ErrnoSuccess {
		return 0, errno
	}
	flags := libc.OpenFileFlags(dirflags, oflags, fdflags, rights)

	// TODO: figure out mode
	f, err := hackpadfs.OpenFile(fsys.fs, path, flags, 0755)
	if err != nil {
		return 0, libc.Error(err)
	}

	fd := fsys.nextfd
	fsys.nextfd++ // TODO: handle overflow

	var desc *filedesc
	desc, errno = newFile(f)
	desc.no = fd
	desc.fdstat.Flags = fdflags
	desc.fdstat.RightsBase = rights

	fsys.fds[fd] = desc
	fsys.share(desc)
	return fd, errno
}

func (fsys *filesystem) close(fd libc.Int) libc.Errno {
	desc, errno := fsys.get(fd)
	if errno != libc.ErrnoSuccess {
		return errno
	}
	fsys.unshare(desc)
	return libc.ErrnoSuccess
}

func (fsys *filesystem) rel(fd int32, name string) (string, libc.Errno) {
	name = cleanPath(name)
	if fd == 0 || (fsys.fs != nil && fd == rootFD) {
		return name, libc.ErrnoSuccess
	}
	f, errno := fsys.get(fd)
	if errno != libc.ErrnoSuccess {
		return "", errno
	}
	name, errno = f.rel(name)
	if errno != libc.ErrnoSuccess {
		return "", errno
	}
	return name, libc.ErrnoSuccess
}

func (fsys *filesystem) readlink(fd int32, name string) (string, libc.Errno) {
	if fsys.fs == nil {
		return "", libc.ErrnoNosys
	}
	name, errno := fsys.rel(fd, name)
	if errno != libc.ErrnoSuccess {
		return "", errno
	}
	info, err := hackpadfs.Stat(fsys.fs, name)
	if err != nil {
		return "", libc.Error(err)
	}
	// TODO: is this accurate?
	return info.Name(), libc.ErrnoSuccess
}

func (fsys *filesystem) rename(fd int32, old, new string) libc.Errno {
	if fsys.fs == nil {
		return libc.ErrnoNosys
	}
	old, errno := fsys.rel(fd, old)
	if errno != libc.ErrnoSuccess {
		return errno
	}
	// TODO: is new also relative?
	err := hackpadfs.Rename(fsys.fs, old, new)
	return libc.Error(err)
}

func (fsys *filesystem) remove(fd int32, name string) libc.Errno {
	if fsys.fs == nil {
		return libc.ErrnoNosys
	}
	name, errno := fsys.rel(fd, name)
	if errno != libc.ErrnoSuccess {
		return errno
	}
	err := hackpadfs.Remove(fsys.fs, name)
	return libc.Error(err)
}

func (fsys *filesystem) rmdir(fd int32, name string) libc.Errno {
	if fsys.fs == nil {
		return libc.ErrnoNosys
	}
	name, errno := fsys.rel(fd, name)
	if errno != libc.ErrnoSuccess {
		return errno
	}
	stat, err := hackpadfs.Stat(fsys.fs, name)
	if err != nil {
		return libc.Error(err)
	}
	if !stat.IsDir() {
		return libc.ErrnoNotdir
	}
	err = hackpadfs.Remove(fsys.fs, name)
	return libc.Error(err)
}

func (fsys *filesystem) mkdir(fd int32, name string, mode fs.FileMode) libc.Errno {
	if fsys.fs == nil {
		return libc.ErrnoNosys
	}
	name, errno := fsys.rel(fd, name)
	if errno != libc.ErrnoSuccess {
		return errno
	}
	err := hackpadfs.Mkdir(fsys.fs, name, mode)
	return libc.Error(err)
}

// func (fsys *filesystem) rmdir(name string) libc.Errno {
// 	if fsys.fs == nil {
// 		return libc.ErrnoNosys
// 	}
// 	err := hackpadfs.RemoveAll(fsys.fs, name)
// 	return libc.Error(err)
// }

func (fsys *filesystem) readdir(fd libc.Int, cookie int64) (ent *libc.Dirent, name string, errno libc.Errno) {
	if fsys.fs == nil {
		return nil, "", libc.ErrnoNosys
	}

	// TODO: use fancy fs ReadDir(n) instead of fake cookie

	i := int(cookie)
	f, errno := fsys.get(fd)
	if errno != 0 {
		return nil, "", errno
	}
	if f.dirent == nil {
		info, err := f.Stat()
		if err != nil {
			return nil, "", libc.Error(err)
		}
		dirname := info.Name()
		f.dirent, err = fs.ReadDir(fsys.fs, dirname)
		if err != nil {
			return nil, "", libc.Error(err)
		}
		i = 0
	}
	if i >= len(f.dirent) {
		return nil, "", libc.ErrnoSuccess
	}
	name = f.dirent[i].Name()
	dtype := libc.FiletypeRegularFile
	if f.dirent[i].IsDir() {
		dtype = libc.FiletypeDirectory
	}
	dir := &libc.Dirent{
		Next:   uint64(i + 1),
		Ino:    fsys.ino(fd, name),
		Namlen: libc.Size(len(name)),
		Dtype:  dtype,
	}
	return dir, name, libc.ErrnoSuccess
}

type filedesc struct {
	fs.File
	no      libc.Int
	fdstat  *libc.Fdstat
	preopen string
	dirent  []fs.DirEntry
	mode    fs.FileMode

	rc int
}

func (fd *filedesc) Write(b []byte) (int, error) {
	if w, ok := fd.File.(io.Writer); ok {
		return w.Write(b)
	}
	return 0, syscall.ENOSYS
}

func (fd *filedesc) rel(name string) (string, libc.Errno) {
	if fd.preopen != "" {
		return fd.preopen + name, libc.ErrnoSuccess
	}
	stat, err := fd.File.Stat()
	if err != nil {
		return "", libc.Error(err)
	}
	return path.Join(stat.Name(), name), libc.ErrnoSuccess
}

func (fsys *filesystem) share(fd *filedesc) {
	if fd.no <= stdioMaxFD {
		return
	}
	fd.rc++
}

func (fsys *filesystem) unshare(fd *filedesc) {
	if fd.no <= stdioMaxFD {
		return
	}
	fd.rc--
	if fd.rc <= 0 {
		// log.Println("gc", fd.no)
		delete(fsys.fds, fd.no)
	}
}

type file interface {
	fs.File
	io.WriteSeeker
}

func newFile(f fs.File) (*filedesc, libc.Errno) {
	stat, err := f.Stat()
	if err != nil {
		return nil, libc.Error(err)
	}

	var fdstat libc.Fdstat
	mode := stat.Mode()
	switch {
	case mode.IsRegular():
		fdstat.Filetype = libc.FiletypeRegularFile
	case mode.IsDir():
		fdstat.Filetype = libc.FiletypeDirectory
	case mode&fs.ModeCharDevice != 0:
		fdstat.Filetype = libc.FiletypeCharacterDevice
	case mode&fs.ModeDevice != 0:
		fdstat.Filetype = libc.FiletypeBlockDevice
	case mode&fs.ModeSymlink != 0:
		fdstat.Filetype = libc.FiletypeSymbolicLink
	case mode&fs.ModeSocket != 0:
		fdstat.Filetype = libc.FiletypeSocketStream
	default:
		fdstat.Filetype = libc.FiletypeUnknown
	}
	fdstat.RightsInheriting = uint64(mode.Perm()) // TODO: verify
	fdstat.RightsBase = uint64(mode.Perm())       // TODO: verify

	return &filedesc{
		File:   f,
		fdstat: &fdstat,
		mode:   mode,
	}, libc.ErrnoSuccess
}

func newStream(v any) *filedesc {
	file := &stream{}
	if x, ok := v.(io.Writer); ok {
		file.Writer = x
	}
	if x, ok := v.(io.Reader); ok {
		file.Reader = x
	}
	if x, ok := v.(io.Closer); ok {
		file.Closer = x
	}
	if x, ok := v.(io.Seeker); ok {
		file.Seeker = x
	}
	if x, ok := v.(statter); ok {
		file.statter = x
	}

	stat := libc.Fdstat{
		Filetype: libc.FiletypeCharacterDevice,
	}

	return &filedesc{
		File:   file,
		fdstat: &stat,
		mode:   fs.ModePerm, // 0777, TODO: fix?
	}
}

// TODO: replace with https://github.com/hack-pad/hackpadfs

type stream struct {
	io.Writer
	io.Closer
	io.Seeker
	io.Reader
	statter
}

type statter interface {
	Stat() (fs.FileInfo, error)
}

func (s *stream) Stat() (fs.FileInfo, error) {
	if s.statter != nil {
		return s.statter.Stat()
	}
	return fileinfo{}, nil // TODO
}

func (s *stream) Write(p []byte) (int, error) {
	if s.Writer == nil {
		return 0, fs.ErrInvalid
	}
	return s.Writer.Write(p)
}

func (s *stream) Seek(offset int64, whence int) (int64, error) {
	if s.Seeker == nil {
		return 0, io.EOF
	}
	return s.Seeker.Seek(offset, whence)
}

func (s *stream) Close() error {
	if s.Closer == nil {
		return nil
	}
	return s.Closer.Close()
}

func (s *stream) Read(buf []byte) (int, error) {
	if s.Reader == nil {
		return 0, io.EOF
	}
	return s.Reader.Read(buf)
}

type fileinfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
	sys     any
}

func (fi fileinfo) Name() string {
	return fi.name
}

func (fi fileinfo) Size() int64 {
	return fi.size
}

func (fi fileinfo) Mode() fs.FileMode {
	return fi.mode
}

func (fi fileinfo) ModTime() time.Time {
	return fi.modTime
}

func (fi fileinfo) IsDir() bool {
	return fi.isDir
}

func (fi fileinfo) Sys() any {
	return fi.sys
}

func cleanPath(name string) string {
	name = path.Clean(name)
	if len(name) > 0 && name[0] == '/' {
		return name[1:]
	}
	return name
}
