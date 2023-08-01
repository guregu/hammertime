package libc

import (
	"syscall"
)

type Filetype = uint8

const (
	// The type of a file descriptor or file is unknown or is different from any of the other types specified.
	FiletypeUnknown Filetype = 0
	// The file descriptor or file refers to a block device inode.
	FiletypeBlockDevice Filetype = 1
	// The file descriptor or file refers to a character device inode.
	FiletypeCharacterDevice Filetype = 2
	// The file descriptor or file refers to a directory inode.
	FiletypeDirectory Filetype = 3
	// The file descriptor or file refers to a regular file inode.
	FiletypeRegularFile Filetype = 4
	// The file descriptor or file refers to a datagram socket.
	FiletypeSocketDgram Filetype = 5
	// The file descriptor or file refers to a byte-stream socket.
	FiletypeSocketStream Filetype = 6
	// The file refers to a symbolic link inode.
	FiletypeSymbolicLink Filetype = 7
)

type Fdflag = uint16

const (
	// Append mode: Data written to the file is always appended to the file's end.
	FdflagAppend Fdflag = 1 << iota
	// Write according to synchronized I/O data integrity completion. Only the data stored in the file is synchronized.
	FdflagDSync
	// Non-blocking mode.
	FdflagNonBlock
	// Synchronized read I/O operations.
	FdflagRSync
	// Write according to synchronized I/O file integrity completion. In addition to synchronizing the data stored in the file, the implementation may also synchronously update the file's metadata.
	FdflagSync
)

type Filestat struct {
	// Device ID of device containing the file.
	Dev uint64
	// File serial number.
	Ino uint64
	// File type.
	Filetype Filetype
	// Number of hard links to the file.
	Nlink uint64
	// For regular files, the file size in bytes. For symbolic links, the length in bytes of the pathname contained in the symbolic link.
	Size uint64
	// Last data access timestamp.
	Atim uint64
	// Last data modification timestamp.
	Mtim uint64
	// Last file status change timestamp.
	Ctim uint64
}

type Ciovec struct {
	Buf Ptr
	Len Size
}

type Iovec struct {
	Buf Ptr
	Len Size
}

type PrestatDir struct {
	Tag    uint8
	DirLen Size
}

type Fdstat struct {
	Filetype         Filetype
	Flags            Fdflag
	RightsBase       uint64
	RightsInheriting uint64
}

type Dirent struct {
	Next   uint64
	Ino    uint64
	Namlen Size
	Dtype  uint8
}

type Lookupflag = Uint

const (
	LookupflagSymlinkfollow Lookupflag = (1 << 0)
)

type Oflag = uint16

const (
	OflagCreat     Oflag = (1 << 0)
	OflagDirectory Oflag = (1 << 1)
	OflagExcl      Oflag = (1 << 2)
	OflagTrunc     Oflag = (1 << 3)
)

// TODO: apparently rights are going away in preview2?

type Rights = uint64

const (
	RightFdDatasync           Rights = (1 << 0)
	RightFdRead               Rights = (1 << 1)
	RightFdSeek               Rights = (1 << 2)
	RightFdFdstatSetFlags     Rights = (1 << 3)
	RightFdSync               Rights = (1 << 4)
	RightFdTell               Rights = (1 << 5)
	RightFdWrite              Rights = (1 << 6)
	RightFdAdvise             Rights = (1 << 7)
	RightFdAllocate           Rights = (1 << 8)
	RightPathCreateDirectory  Rights = (1 << 9)
	RightPathCreateFile       Rights = (1 << 10)
	RightPathLinkSource       Rights = (1 << 11)
	RightPathLinkTarget       Rights = (1 << 12)
	RightPathOpen             Rights = (1 << 13)
	RightFdReaddir            Rights = (1 << 14)
	RightPathReadlink         Rights = (1 << 15)
	RightPathRenameSource     Rights = (1 << 16)
	RightPathRenameTarget     Rights = (1 << 17)
	RightPathFilestatGet      Rights = (1 << 18)
	RightPathFilestatSetSize  Rights = (1 << 19)
	RightPathFilestatSetTimes Rights = (1 << 20)
	RightFdFilestatGet        Rights = (1 << 21)
	RightFdFilestatSetSize    Rights = (1 << 22)
	RightFdFilestatSetTimes   Rights = (1 << 23)
	RightPathSymlink          Rights = (1 << 24)
	RightPathRemoveDirectory  Rights = (1 << 25)
	RightPathUnlinkFile       Rights = (1 << 26)
	RightPollFdReadwrite      Rights = (1 << 27)
	RightSockShutdown         Rights = (1 << 28)
	RightSockAccept           Rights = (1 << 29)
)

func OpenFileFlags(df Lookupflag, of Oflag, fd Fdflag, rights Rights) int {
	// TODO: we probably shouldn't rely on syscall package
	// might need to implement some of the weirder ones like O_DIRECTORY ourselves
	// some are defined in fs.FileMode

	var gf int
	if df&LookupflagSymlinkfollow == 0 {
		gf |= syscall.O_NOFOLLOW
	}
	if of&OflagCreat != 0 {
		gf |= syscall.O_CREAT
	}
	if of&OflagDirectory != 0 {
		gf |= syscall.O_DIRECTORY
	}
	if of&OflagExcl != 0 {
		gf |= syscall.O_EXCL
	}
	if of&OflagTrunc != 0 {
		gf |= syscall.O_TRUNC
	}
	if fd&FdflagAppend != 0 {
		gf |= syscall.O_APPEND
	}
	if fd&FdflagDSync != 0 {
		gf |= syscall.O_DSYNC
	}
	if fd&FdflagNonBlock != 0 {
		gf |= syscall.O_NONBLOCK
	}
	// if fd&FdflagRSync != 0 {
	// 	gf |= syscall.O_RSYNC
	// }
	if fd&FdflagSync != 0 {
		gf |= syscall.O_SYNC
	}

	switch {
	case of&OflagDirectory != 0:
		gf |= syscall.O_RDONLY
	case rights&(RightFdRead&RightFdWrite) != 0:
		gf |= syscall.O_RDWR
	case rights&RightFdWrite != 0:
		gf |= syscall.O_WRONLY
	case rights&RightFdRead != 0:
		// TODO: is this correct?
		if of&(OflagCreat|OflagTrunc) != 0 || fd&FdflagAppend != 0 {
			gf |= syscall.O_RDWR
		} else {
			gf |= syscall.O_RDONLY
		}
	}

	return gf
}
