// Package config resolves Forge configuration without accessing the filesystem.
package config

import (
	"fmt"
	"path/filepath"
)

const databaseName = "forge.db"

// ResolveDatabasePath returns the platform-specific Forge database path using
// injected operating-system and environment values. It performs no filesystem
// access.
func ResolveDatabasePath(goos string, getenv func(string) string) (string, error) {
	if override := getenv("FORGE_DATA_DIR"); override != "" {
		return databasePath("FORGE_DATA_DIR", override)
	}

	switch goos {
	case "linux":
		if dataHome := getenv("XDG_DATA_HOME"); dataHome != "" {
			base, err := absolutePath("XDG_DATA_HOME", dataHome)
			if err != nil {
				return "", err
			}
			return filepath.Join(base, "forge", databaseName), nil
		}
		home, err := requiredHome(getenv)
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", "forge", databaseName), nil
	case "darwin":
		home, err := requiredHome(getenv)
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "forge", databaseName), nil
	default:
		return "", fmt.Errorf("unsupported operating system %q", goos)
	}
}

func databasePath(name, base string) (string, error) {
	base, err := absolutePath(name, base)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, databaseName), nil
}

func requiredHome(getenv func(string) string) (string, error) {
	home := getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("HOME is required to resolve the Forge database path")
	}
	return absolutePath("HOME", home)
}

func absolutePath(name, value string) (string, error) {
	if !filepath.IsAbs(value) {
		return "", fmt.Errorf("%s must be an absolute path", name)
	}
	return value, nil
}
