package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLDriver implements Driver for MySQL using go-sql-driver/mysql.
type MySQLDriver struct {
	db *sql.DB
}

// NewMySQLDriver connects to MySQL using the given DSN
// (e.g. "user:password@tcp(localhost:3306)/dbname").
func NewMySQLDriver(ctx context.Context, dsn string) (*MySQLDriver, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("mysql ping: %w", err)
	}
	return &MySQLDriver{db: db}, nil
}

// Ping implements Driver.
func (d *MySQLDriver) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// ListTables implements Driver. Schema maps to the MySQL database; if empty
// the current database (from the DSN) is used.
func (d *MySQLDriver) ListTables(ctx context.Context, schema string) ([]string, error) {
	var query string
	var args []any
	if schema == "" {
		query = `SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'
			ORDER BY TABLE_NAME`
	} else {
		query = `SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES
			WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'
			ORDER BY TABLE_NAME`
		args = []any{schema}
	}
	rows, err := d.db.QueryContext(ctx, query, args...)
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

// DescribeTable implements Driver. Schema maps to MySQL database; if empty
// the current database is used.
func (d *MySQLDriver) DescribeTable(ctx context.Context, schema, table string) ([]ColumnInfo, error) {
	var query string
	var args []any
	if schema == "" {
		query = `
		SELECT c.COLUMN_NAME, c.DATA_TYPE,
		       c.IS_NULLABLE = 'YES',
		       CASE WHEN c.COLUMN_KEY = 'PRI' THEN 1 ELSE 0 END
		FROM INFORMATION_SCHEMA.COLUMNS c
		WHERE c.TABLE_SCHEMA = DATABASE() AND c.TABLE_NAME = ?
		ORDER BY c.ORDINAL_POSITION`
		args = []any{table}
	} else {
		query = `
		SELECT c.COLUMN_NAME, c.DATA_TYPE,
		       c.IS_NULLABLE = 'YES',
		       CASE WHEN c.COLUMN_KEY = 'PRI' THEN 1 ELSE 0 END
		FROM INFORMATION_SCHEMA.COLUMNS c
		WHERE c.TABLE_SCHEMA = ? AND c.TABLE_NAME = ?
		ORDER BY c.ORDINAL_POSITION`
		args = []any{schema, table}
	}
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		var nullable, isPK int
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &isPK); err != nil {
			return nil, err
		}
		c.Nullable = nullable == 1
		c.IsPK = isPK == 1
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// RunReadOnlyQuery implements Driver. Converts $1, $2 placeholders to MySQL's
// positional ? syntax.
func (d *MySQLDriver) RunReadOnlyQuery(ctx context.Context, query string, params []any) ([]map[string]any, error) {
	query = convertPlaceholdersToMySQL(query)
	rows, err := d.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return sqlRowsToMaps(rows)
}

// convertPlaceholdersToMySQL replaces $1, $2, ... with ? for go-sql-driver/mysql.
// MySQL uses positional ? without numbers; order must match params slice.
func convertPlaceholdersToMySQL(s string) string {
	return dollarPlaceholder.ReplaceAllString(s, "?")
}

// InsertRow implements Driver.
func (d *MySQLDriver) InsertRow(ctx context.Context, schema, table string, row map[string]any) (any, error) {
	if len(row) == 0 {
		return nil, fmt.Errorf("insert row: no columns")
	}
	cols, vals := mapsToColumnsAndValues(row)
	placeholders := makeMySQLPlaceholders(len(cols))
	quotedTable := quoteMySQLTable(schema, table)
	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = quoteMySQLIdentifier(c)
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quotedTable, joinQuoted(quotedCols), placeholders)

	result, err := d.db.ExecContext(ctx, query, vals...)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, nil
	}
	if id > 0 {
		return id, nil
	}
	return nil, nil
}

// UpdateRow implements Driver. Validates key matches actual PK, then updates a single row.
func (d *MySQLDriver) UpdateRow(ctx context.Context, schema, table string, key map[string]any, set map[string]any) (int64, error) {
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

	// Build SET clause: `col1` = ?, `col2` = ?, ...
	setCols, setVals := mapsToColumnsAndValues(set)
	quotedSets := make([]string, len(setCols))
	for i, c := range setCols {
		quotedSets[i] = fmt.Sprintf("%s = ?", quoteMySQLIdentifier(c))
	}

	// Build WHERE clause: `pk1` = ? AND `pk2` = ?, ...
	keyCols, keyVals := mapsToColumnsAndValues(key)
	quotedWheres := make([]string, len(keyCols))
	for i, c := range keyCols {
		quotedWheres[i] = fmt.Sprintf("%s = ?", quoteMySQLIdentifier(c))
	}

	quotedT := quoteMySQLTable(schema, table)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		quotedT,
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

func makeMySQLPlaceholders(n int) string {
	if n == 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

var mysqlIdentReplacer = strings.NewReplacer("`", "``")

func quoteMySQLIdentifier(name string) string {
	return "`" + mysqlIdentReplacer.Replace(name) + "`"
}

// quoteMySQLTable returns `schema`.`table` if schema is non-empty, otherwise `table`.
func quoteMySQLTable(schema, table string) string {
	if schema == "" {
		return quoteMySQLIdentifier(table)
	}
	return quoteMySQLIdentifier(schema) + "." + quoteMySQLIdentifier(table)
}

// Close implements Driver.
func (d *MySQLDriver) Close() error {
	return d.db.Close()
}

var _ Driver = (*MySQLDriver)(nil)
