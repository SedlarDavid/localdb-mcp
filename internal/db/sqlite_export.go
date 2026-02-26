package db

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// sqliteFilePath extracts the filesystem path from a SQLite URI.
// Handles: "/path/to/db.sqlite", "file:/path?mode=rwc", ":memory:".
func sqliteFilePath(uri string) (string, error) {
	if uri == ":memory:" || uri == "" {
		return "", fmt.Errorf("cannot export/import an in-memory SQLite database")
	}
	path := uri
	if strings.HasPrefix(path, "file:") {
		path = strings.TrimPrefix(path, "file:")
		if idx := strings.Index(path, "?"); idx >= 0 {
			path = path[:idx]
		}
	}
	return path, nil
}

// ExportDatabase dumps the SQLite database to a SQL file using sqlite3 .dump.
func (d *SQLiteDriver) ExportDatabase(ctx context.Context, path string) error {
	sqlite3, err := findCLITool("sqlite3")
	if err != nil {
		return err
	}
	absPath, err := validateExportPath(path)
	if err != nil {
		return err
	}
	dbPath, err := sqliteFilePath(d.uri)
	if err != nil {
		return err
	}
	// sqlite3 dbpath .dump > outputfile
	return runCLICaptureStdout(ctx, absPath, sqlite3, dbPath, ".dump")
}

// ImportDatabase loads a SQL dump file into the SQLite database using sqlite3.
func (d *SQLiteDriver) ImportDatabase(ctx context.Context, path string) error {
	sqlite3, err := findCLITool("sqlite3")
	if err != nil {
		return err
	}
	absPath, err := validateImportPath(path)
	if err != nil {
		return err
	}
	dbPath, err := sqliteFilePath(d.uri)
	if err != nil {
		return err
	}

	f, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("import: open file: %w", err)
	}
	defer f.Close()

	return runCLIWithStdin(ctx, nil, f, sqlite3, dbPath)
}

// Ensure SQLiteDriver implements Exporter.
var _ Exporter = (*SQLiteDriver)(nil)
