package mywasi

import (
	"errors"
	"io"
	"io/fs"
	"time"
)

const stdioMaxFD = 3

type filesystem struct {
	fds    map[int_t]*filedesc
	nextfd int_t
	fs     fs.FS
}

func newFilesystem(fsys fs.FS, stdin io.Reader, stdout, stderr io.Writer) *filesystem {
	system := &filesystem{
		fds: map[int32]*filedesc{},
		fs:  fsys,
	}
	system.set(1, newStream(stdin))
	system.set(2, newStream(stdout))
	system.set(3, newStream(stderr))
	return system
}

func (fsys *filesystem) set(no int_t, fd *filedesc) {
	fd.no = no
	fsys.fds[no] = fd
	if fsys.nextfd <= no {
		fsys.nextfd = no + 1
	}
}

func (fsys *filesystem) get(fd int_t) (*filedesc, Errno) {
	f, ok := fsys.fds[fd]
	if !ok {
		return nil, ErrnoBadf
	}
	return f, ErrnoSuccess
}

func (fsys *filesystem) stat(fd int_t) (*fdstat, Errno) {
	f, errno := fsys.get(fd)
	if errno != ErrnoSuccess {
		return nil, errno
	}
	return f.fdstat, ErrnoSuccess
}

func (fsys *filesystem) open(path string) (fd int_t, errno Errno) {
	f, err := fsys.fs.Open(path)
	switch {
	case errors.Is(err, fs.ErrExist):
		return 0, ErrnoNoent
	case errors.Is(err, fs.ErrInvalid):
		return 0, ErrnoInval
	}
	if err != nil {
		panic(err)
	}
	fd = fsys.nextfd
	fsys.nextfd++
	desc := newStream(f)
	fsys.fds[fd] = desc
	fsys.share(desc)
	return
}

func (fsys *filesystem) close(fd int_t) Errno {
	desc, errno := fsys.get(fd)
	if errno != ErrnoSuccess {
		return errno
	}
	fsys.unshare(desc)
	return ErrnoSuccess
}

type filedesc struct {
	File
	no     int_t
	fdstat *fdstat
	rc     int
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

	// if *file == (stream{}) {
	// 	panic("invalid stream")
	// }

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
