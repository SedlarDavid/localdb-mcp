package db

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	_ "github.com/microsoft/go-mssqldb"
)

// SQLServerDriver implements Driver for SQL Server using go-mssqldb.
type SQLServerDriver struct {
	db *sql.DB
}

// NewSQLServerDriver connects to SQL Server using the given URI (e.g. sqlserver://user:pass@host?database=dbname).
func NewSQLServerDriver(ctx context.Context, uri string) (*SQLServerDriver, error) {
	db, err := sql.Open("sqlserver", uri)
	if err != nil {
		return nil, fmt.Errorf("sqlserver open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlserver ping: %w", err)
	}
	return &SQLServerDriver{db: db}, nil
}

// Ping implements Driver.
func (d *SQLServerDriver) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// ListTables implements Driver. Schema is the schema name (e.g. "dbo").
func (d *SQLServerDriver) ListTables(ctx context.Context, schema string) ([]string, error) {
	if schema == "" {
		schema = "dbo"
	}
	sql := `SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = @p1 AND TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME`
	rows, err := d.db.QueryContext(ctx, sql, schema)
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
func (d *SQLServerDriver) DescribeTable(ctx context.Context, schema, table string) ([]ColumnInfo, error) {
	if schema == "" {
		schema = "dbo"
	}
	sql := `
	SELECT c.COLUMN_NAME, c.DATA_TYPE,
	       CASE WHEN c.IS_NULLABLE = 'YES' THEN 1 ELSE 0 END,
	       CASE WHEN pk.COLUMN_NAME IS NOT NULL THEN 1 ELSE 0 END
	FROM INFORMATION_SCHEMA.COLUMNS c
	LEFT JOIN (
	  SELECT ku.TABLE_SCHEMA, ku.TABLE_NAME, ku.COLUMN_NAME
	  FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
	  JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE ku ON tc.CONSTRAINT_NAME = ku.CONSTRAINT_NAME AND tc.TABLE_SCHEMA = ku.TABLE_SCHEMA
	  WHERE tc.CONSTRAINT_TYPE = 'PRIMARY KEY'
	) pk ON c.TABLE_SCHEMA = pk.TABLE_SCHEMA AND c.TABLE_NAME = pk.TABLE_NAME AND c.COLUMN_NAME = pk.COLUMN_NAME
	WHERE c.TABLE_SCHEMA = @p1 AND c.TABLE_NAME = @p2
	ORDER BY c.ORDINAL_POSITION`
	rows, err := d.db.QueryContext(ctx, sql, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		var nullableInt, isPK int
		if err := rows.Scan(&c.Name, &c.Type, &nullableInt, &isPK); err != nil {
			return nil, err
		}
		c.Nullable = nullableInt == 1
		c.IsPK = isPK == 1
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// RunReadOnlyQuery implements Driver. Converts $1, $2 placeholders to @p1, @p2 for SQL Server.
func (d *SQLServerDriver) RunReadOnlyQuery(ctx context.Context, sql string, params []any) ([]map[string]any, error) {
	sql = convertPlaceholdersToMSSQL(sql)
	rows, err := d.db.QueryContext(ctx, sql, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return sqlRowsToMaps(rows)
}

// convertPlaceholdersToMSSQL replaces $1, $2, ... with @p1, @p2, ... for go-mssqldb.
func convertPlaceholdersToMSSQL(s string) string {
	return dollarPlaceholder.ReplaceAllString(s, "@p${1}")
}

var dollarPlaceholder = regexp.MustCompile(`\$(\d+)`)

// sqlRowsToMaps builds []map[string]any from database/sql.Rows.
func sqlRowsToMaps(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, nil
	}
	var out []map[string]any
	scan := make([]any, len(cols))
	for i := range scan {
		scan[i] = new(any)
	}
	for rows.Next() {
		if err := rows.Scan(scan...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			m[c] = *(scan[i].(*any))
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// InsertRow implements Driver. Uses OUTPUT INSERTED.<first_identity> to return generated ID when possible.
func (d *SQLServerDriver) InsertRow(ctx context.Context, schema, table string, row map[string]any) (any, error) {
	if schema == "" {
		schema = "dbo"
	}
	if len(row) == 0 {
		return nil, fmt.Errorf("insert row: no columns")
	}
	cols, vals := mapsToColumnsAndValues(row)
	placeholders := makeMSSQLPlaceholders(len(cols))
	quotedTable := quoteMSSQLIdentifier(schema) + "." + quoteMSSQLIdentifier(table)
	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = quoteMSSQLIdentifier(c)
	}
	sql := fmt.Sprintf("INSERT INTO %s (%s) OUTPUT INSERTED.* VALUES (%s)",
		quotedTable, joinQuoted(quotedCols), placeholders)
	params := make([]any, len(vals))
	copy(params, vals)
	rows, err := d.db.QueryContext(ctx, sql, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	outCols, _ := rows.Columns()
	scan := make([]any, len(outCols))
	for i := range scan {
		scan[i] = new(any)
	}
	if err := rows.Scan(scan...); err != nil {
		return nil, err
	}
	if len(scan) > 0 {
		return *(scan[0].(*any)), nil
	}
	return nil, nil
}

// UpdateRow implements Driver. Validates key matches actual PK, then updates a single row.
func (d *SQLServerDriver) UpdateRow(ctx context.Context, schema, table string, key map[string]any, set map[string]any) (int64, error) {
	if schema == "" {
		schema = "dbo"
	}
	if len(key) == 0 {
		return 0, fmt.Errorf("update row: key must contain at least one column")
	}
	if len(set) == 0 {
		return 0, fmt.Errorf("update row: set must contain at least one column")
	}

	// Fetch actual PK columns and validate the provided key matches.
	if err := validatePKColumns(ctx, d, schema, table, key); err != nil {
		return 0, err
	}

	// Build SET clause: [col1] = @p1, [col2] = @p2, ...
	setCols, setVals := mapsToColumnsAndValues(set)
	quotedSets := make([]string, len(setCols))
	for i, c := range setCols {
		quotedSets[i] = fmt.Sprintf("%s = @p%d", quoteMSSQLIdentifier(c), i+1)
	}

	// Build WHERE clause: [pk1] = @pN AND [pk2] = @pN+1, ...
	keyCols, keyVals := mapsToColumnsAndValues(key)
	quotedWheres := make([]string, len(keyCols))
	for i, c := range keyCols {
		quotedWheres[i] = fmt.Sprintf("%s = @p%d", quoteMSSQLIdentifier(c), len(setCols)+i+1)
	}

	quotedTable := quoteMSSQLIdentifier(schema) + "." + quoteMSSQLIdentifier(table)
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

func makeMSSQLPlaceholders(n int) string {
	if n == 0 {
		return ""
	}
	b := ""
	for i := 1; i <= n; i++ {
		if i > 1 {
			b += ", "
		}
		b += fmt.Sprintf("@p%d", i)
	}
	return b
}

var mssqlIdentReplacer = strings.NewReplacer("]", "]]")

func quoteMSSQLIdentifier(name string) string {
	return "[" + mssqlIdentReplacer.Replace(name) + "]"
}

// Close implements Driver.
func (d *SQLServerDriver) Close() error {
	return d.db.Close()
}

var _ Driver = (*SQLServerDriver)(nil)
