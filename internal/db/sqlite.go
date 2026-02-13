package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// SQLiteDriver implements Driver for SQLite using modernc.org/sqlite (pure Go, no CGO).
type SQLiteDriver struct {
	db *sql.DB
}

// NewSQLiteDriver opens a SQLite database at the given path (or URI such as "file:path?mode=...").
func NewSQLiteDriver(_ context.Context, uri string) (*SQLiteDriver, error) {
	db, err := sql.Open("sqlite", uri)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite ping: %w", err)
	}
	return &SQLiteDriver{db: db}, nil
}

// Ping implements Driver.
func (d *SQLiteDriver) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// ListTables implements Driver. Schema is ignored for SQLite (single schema).
func (d *SQLiteDriver) ListTables(ctx context.Context, _ string) ([]string, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// DescribeTable implements Driver.
func (d *SQLiteDriver) DescribeTable(ctx context.Context, _, table string) ([]ColumnInfo, error) {
	// table_info returns: cid, name, type, notnull, dflt_value, pk
	rows, err := d.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", quoteSQLiteIdentifier(table)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []ColumnInfo
	for rows.Next() {
		var cid int
		var name, colType string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, ColumnInfo{
			Name:     name,
			Type:     colType,
			Nullable: notnull == 0,
			IsPK:     pk > 0,
		})
	}
	return cols, rows.Err()
}

// RunReadOnlyQuery implements Driver. Uses $1, $2 style positional params
// converted to SQLite's ?1, ?2 syntax.
func (d *SQLiteDriver) RunReadOnlyQuery(ctx context.Context, query string, params []any) ([]map[string]any, error) {
	query = convertPlaceholdersToSQLite(query)
	rows, err := d.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return sqlRowsToMaps(rows)
}

// convertPlaceholdersToSQLite replaces $1, $2, ... with ?1, ?2, ... for SQLite.
func convertPlaceholdersToSQLite(s string) string {
	return dollarPlaceholder.ReplaceAllString(s, "?${1}")
}

// InsertRow implements Driver.
func (d *SQLiteDriver) InsertRow(ctx context.Context, _, table string, row map[string]any) (any, error) {
	if len(row) == 0 {
		return nil, fmt.Errorf("insert row: no columns")
	}
	cols, vals := mapsToColumnsAndValues(row)
	placeholders := makeSQLitePlaceholders(len(cols))
	quotedTable := quoteSQLiteIdentifier(table)
	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = quoteSQLiteIdentifier(c)
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quotedTable, joinQuoted(quotedCols), placeholders)

	result, err := d.db.ExecContext(ctx, query, vals...)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	if id > 0 {
		return id, nil
	}
	return nil, nil
}

// UpdateRow implements Driver. Validates key matches actual PK, then updates a single row.
func (d *SQLiteDriver) UpdateRow(ctx context.Context, _, table string, key map[string]any, set map[string]any) (int64, error) {
	if len(key) == 0 {
		return 0, fmt.Errorf("update row: key must contain at least one column")
	}
	if len(set) == 0 {
		return 0, fmt.Errorf("update row: set must contain at least one column")
	}

	// Fetch actual PK columns and validate the provided key matches.
	if err := validatePKColumns(ctx, d, "", table, key); err != nil {
		return 0, err
	}

	// Build SET clause: "col1" = ?1, "col2" = ?2, ...
	setCols, setVals := mapsToColumnsAndValues(set)
	quotedSets := make([]string, len(setCols))
	for i, c := range setCols {
		quotedSets[i] = fmt.Sprintf("%s = ?%d", quoteSQLiteIdentifier(c), i+1)
	}

	// Build WHERE clause: "pk1" = ?N AND "pk2" = ?N+1, ...
	keyCols, keyVals := mapsToColumnsAndValues(key)
	quotedWheres := make([]string, len(keyCols))
	for i, c := range keyCols {
		quotedWheres[i] = fmt.Sprintf("%s = ?%d", quoteSQLiteIdentifier(c), len(setCols)+i+1)
	}

	quotedTable := quoteSQLiteIdentifier(table)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		quotedTable,
		strings.Join(quotedSets, ", "),
		strings.Join(quotedWheres, " AND "),
	)

	params := make([]any, 0, len(setVals)+len(keyVals))
	params = append(params, setVals...)
	params = append(params, keyVals...)

	result, err := d.db.ExecContext(ctx, query, params...)
	if err != nil {
		return 0, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, fmt.Errorf("update row: no row found with the given key")
	}
	return n, nil
}

func makeSQLitePlaceholders(n int) string {
	if n == 0 {
		return ""
	}
	b := ""
	for i := 1; i <= n; i++ {
		if i > 1 {
			b += ", "
		}
		b += fmt.Sprintf("?%d", i)
	}
	return b
}

var sqliteIdentReplacer = strings.NewReplacer(`"`, `""`)

func quoteSQLiteIdentifier(name string) string {
	return `"` + sqliteIdentReplacer.Replace(name) + `"`
}

// Close implements Driver.
func (d *SQLiteDriver) Close() error {
	return d.db.Close()
}

var _ Driver = (*SQLiteDriver)(nil)
