package mywasi

import (
	"io"
	"io/fs"
	"log"
	"path"
	"time"

	libc "github.com/guregu/hammertime/libc"
)

const stdioMaxFD = 3

type filesystem struct {
	fds    map[int_t]*filedesc
	nextfd int_t
	fs     fs.FS
	dev    uint64
}

func newFilesystem(fsys fs.FS, stdin io.Reader, stdout, stderr io.Writer) *filesystem {
	system := &filesystem{
		fds: map[int32]*filedesc{},
		fs:  fsys,
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
			fdstat:  &fdstat{fs_filetype: filetypeDirectory},
			preopen: "/",
		}
		system.set(3, fd3)
	}

	return system
}

func (fsys *filesystem) set(no int_t, fd *filedesc) {
	fd.no = no
	fsys.fds[no] = fd
	if fsys.nextfd <= no {
		fsys.nextfd = no + 1
	}
}

func (fsys *filesystem) get(fd int_t) (*filedesc, libc.Errno) {
	f, ok := fsys.fds[fd]
	if !ok {
		return nil, libc.ErrnoBadf
	}
	return f, libc.ErrnoSuccess
}

func (fsys *filesystem) fdstat(fd int_t) (*fdstat, libc.Errno) {
	f, errno := fsys.get(fd)
	if errno != libc.ErrnoSuccess {
		return nil, errno
	}
	return f.fdstat, libc.ErrnoSuccess
}

func (fsys *filesystem) stat(fd int_t) (*filestat, libc.Errno) {
	f, errno := fsys.get(fd)
	if errno != libc.ErrnoSuccess {
		return nil, errno
	}
	stat, err := f.Stat()
	if err != nil {
		return nil, libc.Error(err)
	}

	fstat := &filestat{
		dev:   fsys.dev,
		ino:   1337, // TODO!!
		mtim:  uint64(stat.ModTime().UnixNano()),
		nlink: 1,
		size:  uint64(stat.Size()),
	}
	if stat.IsDir() {
		fstat.filetype = filetypeDirectory
	} else {
		fstat.filetype = filetypeRegularFile // TODO
	}
	return fstat, libc.ErrnoSuccess
}

func (fsys *filesystem) open(path string) (fd int_t, errno libc.Errno) {
	if fsys.fs == nil {
		return 0, libc.ErrnoNosys
	}

	f, err := fsys.fs.Open(path)
	if err != nil {
		return 0, libc.Error(err)
	}
	fd = fsys.nextfd
	fsys.nextfd++ // TODO: handle overflow
	desc := newStream(f)
	desc.no = fd
	fsys.fds[fd] = desc
	fsys.share(desc)
	return
}

func (fsys *filesystem) close(fd int_t) libc.Errno {
	desc, errno := fsys.get(fd)
	if errno != libc.ErrnoSuccess {
		return errno
	}
	fsys.unshare(desc)
	return libc.ErrnoSuccess
}

func (fsys *filesystem) readlink(name string) (string, libc.Errno) {
	if fsys.fs == nil {
		return "", libc.ErrnoNosys
	}
	name = path.Clean(name)
	f, err := fsys.fs.Open(name)
	if err != nil {
		return "", libc.Error(err)
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return "", libc.Error(err)
	}
	// TODO: mess with name
	return stat.Name(), libc.ErrnoSuccess
}

func (fsys *filesystem) rename(old, new string) libc.Errno {
	if fsys.fs == nil {
		return libc.ErrnoNosys
	}
	// TODO
	return libc.ErrnoNosys
}

func (fsys *filesystem) readdir(fd int_t, cookie int64) (ent *dirent, name string, errno libc.Errno) {
	if fsys.fs == nil {
		return nil, "", libc.ErrnoNosys
	}

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
	dtype := filetypeRegularFile
	if f.dirent[i].IsDir() {
		dtype = filetypeDirectory
	}
	dir := &dirent{
		next:   uint64(i + 1),
		ino:    uint64(1337 + i), // TODO
		namlen: size_t(len(name)),
		dtype:  dtype,
	}
	return dir, name, libc.ErrnoSuccess
}

type filedesc struct {
	File
	no      int_t
	fdstat  *fdstat
	preopen string
	dirent  []fs.DirEntry

	rc int
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
		log.Println("gc", fd.no)
		delete(fsys.fds, fd.no)
	}
}

type File interface {
	fs.File
	io.WriteSeeker
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

	stat := fdstat{
		fs_filetype: filetypeCharacterDevice,
	}

	return &filedesc{
		File:   file,
		fdstat: &stat,
	}
}

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
		return 0, io.EOF
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
