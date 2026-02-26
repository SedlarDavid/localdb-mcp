package db

import "context"

// ExportDatabase dumps the PostgreSQL database to a SQL file using pg_dump.
func (d *PostgresDriver) ExportDatabase(ctx context.Context, path string) error {
	pgDump, err := findCLITool("pg_dump")
	if err != nil {
		return err
	}
	absPath, err := validateExportPath(path)
	if err != nil {
		return err
	}
	// pg_dump accepts the connection URI directly as a positional argument.
	return runCLI(ctx, pgDump,
		d.uri,
		"--file", absPath,
		"--format", "plain",
		"--no-owner",
		"--no-acl",
	)
}

// ImportDatabase loads a SQL dump file into the PostgreSQL database using psql.
func (d *PostgresDriver) ImportDatabase(ctx context.Context, path string) error {
	psql, err := findCLITool("psql")
	if err != nil {
		return err
	}
	absPath, err := validateImportPath(path)
	if err != nil {
		return err
	}
	return runCLI(ctx, psql,
		d.uri,
		"--file", absPath,
		"--quiet",
		"--set", "ON_ERROR_STOP=1",
	)
}

// Ensure PostgresDriver implements Exporter.
var _ Exporter = (*PostgresDriver)(nil)
