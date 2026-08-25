//go:build linux || darwin

package storage

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/sys/unix"
	"modernc.org/sqlite"
)

const BusyTimeoutMS = 250

// OpenSQLite opens a single-connection SQLite pool for a database file that was
// already securely opened relative to directory. SQLite is never allowed to
// create the file: DatabaseCreate means the verified file was created by
// OpenDatabaseFile and is opened read-write here.
//
// The caller retains ownership of directory and database and must keep both
// handles open until the returned database is closed.
func OpenSQLite(ctx context.Context, directory, database *os.File, mode DatabaseOpenMode) (*sql.DB, error) {
	if directory == nil {
		return nil, fmt.Errorf("data directory handle is required")
	}
	if database == nil {
		return nil, fmt.Errorf("database file handle is required")
	}

	sqliteMode, err := sqliteOpenMode(mode)
	if err != nil {
		return nil, err
	}
	directoryInfo, err := directory.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect data directory handle: %w", err)
	}
	if !directoryInfo.IsDir() {
		return nil, fmt.Errorf("data directory handle is not a directory")
	}
	databaseInfo, err := database.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect database file handle: %w", err)
	}
	if !databaseInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("database file handle is not a regular file")
	}
	if !filepath.IsAbs(directory.Name()) {
		return nil, fmt.Errorf("data directory handle path must be absolute")
	}
	if err := verifyDatabaseIdentity(directory, database); err != nil {
		return nil, err
	}

	dsn := sqliteDSN(filepath.Join(directory.Name(), databaseFilename), sqliteMode)
	connector, err := sqlite.NewConnector(dsn)
	if err != nil {
		return nil, fmt.Errorf("configure sqlite connection: %w", err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	if err := verifyDatabaseIdentity(directory, database); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func sqliteOpenMode(mode DatabaseOpenMode) (string, error) {
	switch mode {
	case DatabaseReadOnly:
		return "ro", nil
	case DatabaseReadWrite, DatabaseCreate:
		return "rw", nil
	default:
		return "", fmt.Errorf("invalid database open mode %d", mode)
	}
}

func sqliteDSN(path, mode string) string {
	dsn := &url.URL{Scheme: "file", Path: path}
	query := dsn.Query()
	query.Set("mode", mode)
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", "busy_timeout("+strconv.Itoa(BusyTimeoutMS)+")")
	dsn.RawQuery = query.Encode()
	return dsn.String()
}

func verifyDatabaseIdentity(directory, database *os.File) error {
	var expected unix.Stat_t
	if err := unix.Fstat(int(database.Fd()), &expected); err != nil {
		return fmt.Errorf("inspect verified database file: %w", err)
	}
	var opened unix.Stat_t
	if err := unix.Fstatat(int(directory.Fd()), databaseFilename, &opened, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fmt.Errorf("reinspect sqlite database file: %w", err)
	}
	if opened.Mode&unix.S_IFMT != unix.S_IFREG || expected.Dev != opened.Dev || expected.Ino != opened.Ino {
		return fmt.Errorf("sqlite database file changed during opening")
	}
	return nil
}
