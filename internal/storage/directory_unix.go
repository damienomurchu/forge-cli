//go:build linux || darwin

// Package storage provides secure filesystem primitives for Forge persistence.
package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const privateDirectoryMode = 0o700

// PrepareDataDirectory creates the final component of path when necessary,
// securely opens it, verifies that it belongs to effectiveUID, and restricts
// its permissions to 0700. The parent directory must already exist.
//
// The returned handle pins the verified directory for later descriptor-relative
// operations. The caller must close it.
func PrepareDataDirectory(path string, effectiveUID int) (*os.File, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("data directory path must be absolute")
	}
	if effectiveUID < 0 {
		return nil, fmt.Errorf("effective user ID must not be negative")
	}

	path = filepath.Clean(path)
	parentPath, name := filepath.Split(path)
	if name == "" {
		return nil, fmt.Errorf("data directory path must name a directory")
	}

	parentFD, err := unix.Open(filepath.Clean(parentPath), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open data directory parent: %w", err)
	}
	defer unix.Close(parentFD)

	if err := unix.Mkdirat(parentFD, name, privateDirectoryMode); err != nil && err != unix.EEXIST {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	var pathInfo unix.Stat_t
	if err := unix.Fstatat(parentFD, name, &pathInfo, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return nil, fmt.Errorf("inspect data directory: %w", err)
	}
	if pathInfo.Mode&unix.S_IFMT == unix.S_IFLNK {
		return nil, fmt.Errorf("data directory must not be a symbolic link")
	}
	if pathInfo.Mode&unix.S_IFMT != unix.S_IFDIR {
		return nil, fmt.Errorf("data directory path is not a directory")
	}

	directoryFD, err := unix.Openat(parentFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("securely open data directory: %w", err)
	}

	var directoryInfo unix.Stat_t
	if err := unix.Fstat(directoryFD, &directoryInfo); err != nil {
		unix.Close(directoryFD)
		return nil, fmt.Errorf("inspect open data directory: %w", err)
	}
	if directoryInfo.Uid != uint32(effectiveUID) {
		unix.Close(directoryFD)
		return nil, fmt.Errorf("data directory is owned by user ID %d, want %d", directoryInfo.Uid, effectiveUID)
	}
	if directoryInfo.Mode&0o7777 != privateDirectoryMode {
		if err := unix.Fchmod(directoryFD, privateDirectoryMode); err != nil {
			unix.Close(directoryFD)
			return nil, fmt.Errorf("restrict data directory permissions: %w", err)
		}
	}

	directory := os.NewFile(uintptr(directoryFD), path)
	if directory == nil {
		unix.Close(directoryFD)
		return nil, fmt.Errorf("create data directory handle")
	}
	return directory, nil
}
