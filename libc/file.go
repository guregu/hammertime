package libc

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
