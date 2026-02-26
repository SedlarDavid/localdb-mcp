package db

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// sqlserverConnInfo holds parsed SQL Server URI components for CLI tool usage.
type sqlserverConnInfo struct {
	User     string
	Password string
	Host     string
	Port     string
	Database string
}

// parseSQLServerURI extracts connection components from a SQL Server URI.
// Format: sqlserver://user:pass@host:port?database=dbname
func parseSQLServerURI(uri string) (*sqlserverConnInfo, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("cannot parse SQL Server URI: %w", err)
	}
	info := &sqlserverConnInfo{
		User:     u.User.Username(),
		Host:     u.Hostname(),
		Port:     u.Port(),
		Database: u.Query().Get("database"),
	}
	info.Password, _ = u.User.Password()
	if info.Port == "" {
		info.Port = "1433"
	}
	if info.Database == "" {
		info.Database = "master"
	}
	return info, nil
}

// ExportDatabase dumps the SQL Server database to a SQL file.
// Uses pure Go: queries INFORMATION_SCHEMA to generate CREATE TABLE + INSERT statements.
func (d *SQLServerDriver) ExportDatabase(ctx context.Context, path string) error {
	absPath, err := validateExportPath(path)
	if err != nil {
		return err
	}

	tables, err := d.ListTables(ctx, "dbo")
	if err != nil {
		return fmt.Errorf("export: list tables: %w", err)
	}

	f, err := os.Create(absPath)
	if err != nil {
		return fmt.Errorf("export: create file: %w", err)
	}
	defer f.Close()

	fmt.Fprintf(f, "-- SQL Server database export\n\n")

	for _, table := range tables {
		// Generate CREATE TABLE
		createSQL, err := d.generateCreateTable(ctx, "dbo", table)
		if err != nil {
			return fmt.Errorf("export: generate DDL for %s: %w", table, err)
		}
		fmt.Fprintf(f, "%s\nGO\n\n", createSQL)

		// Generate INSERT statements
		if err := d.generateInserts(ctx, f, "dbo", table); err != nil {
			return fmt.Errorf("export: generate inserts for %s: %w", table, err)
		}
		fmt.Fprintf(f, "\n")
	}

	return nil
}

// generateCreateTable builds a CREATE TABLE statement from INFORMATION_SCHEMA.
func (d *SQLServerDriver) generateCreateTable(ctx context.Context, schema, table string) (string, error) {
	cols, err := d.DescribeTable(ctx, schema, table)
	if err != nil {
		return "", err
	}
	if len(cols) == 0 {
		return "", fmt.Errorf("table %s has no columns", table)
	}

	var b strings.Builder
	quotedTable := quoteMSSQLIdentifier(schema) + "." + quoteMSSQLIdentifier(table)
	fmt.Fprintf(&b, "CREATE TABLE %s (\n", quotedTable)

	var pkCols []string
	for i, c := range cols {
		if i > 0 {
			b.WriteString(",\n")
		}
		nullable := "NOT NULL"
		if c.Nullable {
			nullable = "NULL"
		}
		fmt.Fprintf(&b, "    %s %s %s", quoteMSSQLIdentifier(c.Name), c.Type, nullable)
		if c.IsPK {
			pkCols = append(pkCols, quoteMSSQLIdentifier(c.Name))
		}
	}

	if len(pkCols) > 0 {
		fmt.Fprintf(&b, ",\n    PRIMARY KEY (%s)", strings.Join(pkCols, ", "))
	}

	b.WriteString("\n);")
	return b.String(), nil
}

// generateInserts writes INSERT statements for all rows in a table.
func (d *SQLServerDriver) generateInserts(ctx context.Context, f *os.File, schema, table string) error {
	quotedTable := quoteMSSQLIdentifier(schema) + "." + quoteMSSQLIdentifier(table)
	query := fmt.Sprintf("SELECT * FROM %s", quotedTable)

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return err
	}
	if len(colNames) == 0 {
		return nil
	}

	quotedCols := make([]string, len(colNames))
	for i, c := range colNames {
		quotedCols[i] = quoteMSSQLIdentifier(c)
	}
	colList := strings.Join(quotedCols, ", ")

	scan := make([]any, len(colNames))
	for i := range scan {
		scan[i] = new(any)
	}

	for rows.Next() {
		if err := rows.Scan(scan...); err != nil {
			return err
		}
		vals := make([]string, len(scan))
		for i := range scan {
			v := *(scan[i].(*any))
			vals[i] = formatSQLValue(v)
		}
		fmt.Fprintf(f, "INSERT INTO %s (%s) VALUES (%s);\n",
			quotedTable, colList, strings.Join(vals, ", "))
	}

	if err := rows.Err(); err != nil {
		return err
	}
	fmt.Fprintf(f, "GO\n")
	return nil
}

// formatSQLValue formats a Go value as a SQL literal for INSERT statements.
func formatSQLValue(v any) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "1"
		}
		return "0"
	case []byte:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(string(val), "'", "''"))
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(val, "'", "''"))
	default:
		s := fmt.Sprintf("%v", val)
		return fmt.Sprintf("'%s'", strings.ReplaceAll(s, "'", "''"))
	}
}

// ImportDatabase loads a SQL dump file into the SQL Server database using sqlcmd.
func (d *SQLServerDriver) ImportDatabase(ctx context.Context, path string) error {
	sqlcmd, err := findCLITool("sqlcmd")
	if err != nil {
		return err
	}
	absPath, err := validateImportPath(path)
	if err != nil {
		return err
	}
	info, err := parseSQLServerURI(d.uri)
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}
	return runCLI(ctx, sqlcmd,
		"-S", fmt.Sprintf("%s,%s", info.Host, info.Port),
		"-U", info.User,
		"-P", info.Password,
		"-d", info.Database,
		"-i", absPath,
	)
}

// Ensure SQLServerDriver implements Exporter.
var _ Exporter = (*SQLServerDriver)(nil)
