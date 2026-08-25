//go:build linux || darwin

package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const (
	databaseFilename = "forge.db"
	privateFileMode  = 0o600
)

// DatabaseOpenMode controls whether a database is writable and whether a
// missing database may be created.
type DatabaseOpenMode uint8

const (
	DatabaseReadOnly DatabaseOpenMode = iota
	DatabaseReadWrite
	DatabaseCreate
)

// OpenDatabaseFile securely opens forge.db relative to a previously verified
// data-directory handle. DatabaseCreate creates a missing file atomically with
// mode 0600; the other modes never create it. The caller must close the returned
// handle.
func OpenDatabaseFile(directory *os.File, mode DatabaseOpenMode, effectiveUID int) (*os.File, error) {
	if directory == nil {
		return nil, fmt.Errorf("data directory handle is required")
	}
	if effectiveUID < 0 {
		return nil, fmt.Errorf("effective user ID must not be negative")
	}

	flags, create, err := databaseOpenFlags(mode)
	if err != nil {
		return nil, err
	}

	directoryFD := int(directory.Fd())
	var pathInfo unix.Stat_t
	if err := unix.Fstatat(directoryFD, databaseFilename, &pathInfo, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		if !create || !errors.Is(err, unix.ENOENT) {
			return nil, fmt.Errorf("inspect database file: %w", err)
		}
	} else if pathInfo.Mode&unix.S_IFMT == unix.S_IFLNK {
		return nil, fmt.Errorf("database file must not be a symbolic link")
	} else if pathInfo.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, fmt.Errorf("database path is not a regular file")
	}

	databaseFD, err := unix.Openat(directoryFD, databaseFilename, flags|unix.O_NOFOLLOW|unix.O_CLOEXEC, privateFileMode)
	if err != nil {
		return nil, fmt.Errorf("securely open database file: %w", err)
	}

	var databaseInfo unix.Stat_t
	if err := unix.Fstat(databaseFD, &databaseInfo); err != nil {
		unix.Close(databaseFD)
		return nil, fmt.Errorf("inspect open database file: %w", err)
	}
	if databaseInfo.Mode&unix.S_IFMT != unix.S_IFREG {
		unix.Close(databaseFD)
		return nil, fmt.Errorf("database path is not a regular file")
	}
	if databaseInfo.Uid != uint32(effectiveUID) {
		unix.Close(databaseFD)
		return nil, fmt.Errorf("database file is owned by user ID %d, want %d", databaseInfo.Uid, effectiveUID)
	}
	if databaseInfo.Mode&0o7777 != privateFileMode {
		if err := unix.Fchmod(databaseFD, privateFileMode); err != nil {
			unix.Close(databaseFD)
			return nil, fmt.Errorf("restrict database file permissions: %w", err)
		}
	}

	database := os.NewFile(uintptr(databaseFD), filepath.Join(directory.Name(), databaseFilename))
	if database == nil {
		unix.Close(databaseFD)
		return nil, fmt.Errorf("create database file handle")
	}
	return database, nil
}

func databaseOpenFlags(mode DatabaseOpenMode) (flags int, create bool, err error) {
	switch mode {
	case DatabaseReadOnly:
		return unix.O_RDONLY, false, nil
	case DatabaseReadWrite:
		return unix.O_RDWR, false, nil
	case DatabaseCreate:
		return unix.O_RDWR | unix.O_CREAT, true, nil
	default:
		return 0, false, fmt.Errorf("invalid database open mode %d", mode)
	}
}
