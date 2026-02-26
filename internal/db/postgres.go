package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// PostgresDriver implements Driver for PostgreSQL using pgx.
type PostgresDriver struct {
	conn *pgx.Conn
	uri  string
}

// NewPostgresDriver connects to PostgreSQL using the given URI.
func NewPostgresDriver(ctx context.Context, uri string) (*PostgresDriver, error) {
	conn, err := pgx.Connect(ctx, uri)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}
	return &PostgresDriver{conn: conn, uri: uri}, nil
}

// Ping implements Driver.
func (d *PostgresDriver) Ping(ctx context.Context) error {
	return d.conn.Ping(ctx)
}

// ListTables implements Driver. Schema defaults to "public" if empty.
func (d *PostgresDriver) ListTables(ctx context.Context, schema string) ([]string, error) {
	if schema == "" {
		schema = "public"
	}
	rows, err := d.conn.Query(ctx,
		`SELECT table_name FROM information_schema.tables
		 WHERE table_schema = $1 AND table_type = 'BASE TABLE'
		 ORDER BY table_name`,
		schema)
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
func (d *PostgresDriver) DescribeTable(ctx context.Context, schema, table string) ([]ColumnInfo, error) {
	if schema == "" {
		schema = "public"
	}
	rows, err := d.conn.Query(ctx, `
		SELECT c.column_name, c.data_type, c.is_nullable = 'YES',
		       EXISTS (
		         SELECT 1 FROM information_schema.table_constraints tc
		         JOIN information_schema.key_column_usage kcu
		           ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		         WHERE tc.table_schema = c.table_schema AND tc.table_name = c.table_name
		           AND tc.constraint_type = 'PRIMARY KEY' AND kcu.column_name = c.column_name
		       )
		FROM information_schema.columns c
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position`,
		schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		if err := rows.Scan(&c.Name, &c.Type, &c.Nullable, &c.IsPK); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// RunReadOnlyQuery implements Driver. Params are positional ($1, $2, ...).
func (d *PostgresDriver) RunReadOnlyQuery(ctx context.Context, sql string, params []any) ([]map[string]any, error) {
	rows, err := d.conn.Query(ctx, sql, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return rowsToMaps(rows)
}

func rowsToMaps(rows pgx.Rows) ([]map[string]any, error) {
	fields := rows.FieldDescriptions()
	if len(fields) == 0 {
		return nil, nil
	}
	var out []map[string]any
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		m := make(map[string]any, len(fields))
		for i, f := range fields {
			name := string(f.Name)
			if name == "" {
				name = fmt.Sprintf("column_%d", i+1)
			}
			m[name] = vals[i]
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// InsertRow implements Driver. Returns the value of a single RETURNING column if present.
func (d *PostgresDriver) InsertRow(ctx context.Context, schema, table string, row map[string]any) (any, error) {
	if schema == "" {
		schema = "public"
	}
	if len(row) == 0 {
		return nil, fmt.Errorf("insert row: no columns")
	}
	cols, vals := mapsToColumnsAndValues(row)
	placeholders := makePlaceholders(len(cols))
	quotedTable := pgx.Identifier{schema, table}.Sanitize()
	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = pgx.Identifier{c}.Sanitize()
	}
	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING *",
		quotedTable, joinQuoted(quotedCols), placeholders)
	params := append([]any(nil), vals...)
	rows, err := d.conn.Query(ctx, sql, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	returnVals, err := rows.Values()
	if err != nil {
		return nil, err
	}
	if len(returnVals) > 0 {
		return returnVals[0], nil
	}
	return nil, nil
}

// UpdateRow implements Driver. Validates key matches actual PK, then updates a single row.
func (d *PostgresDriver) UpdateRow(ctx context.Context, schema, table string, key map[string]any, set map[string]any) (int64, error) {
	if schema == "" {
		schema = "public"
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

	// Build SET clause: "col1" = $1, "col2" = $2, ...
	setCols, setVals := mapsToColumnsAndValues(set)
	quotedSets := make([]string, len(setCols))
	for i, c := range setCols {
		quotedSets[i] = fmt.Sprintf("%s = $%d", pgx.Identifier{c}.Sanitize(), i+1)
	}

	// Build WHERE clause: "pk1" = $N AND "pk2" = $N+1, ...
	keyCols, keyVals := mapsToColumnsAndValues(key)
	quotedWheres := make([]string, len(keyCols))
	for i, c := range keyCols {
		quotedWheres[i] = fmt.Sprintf("%s = $%d", pgx.Identifier{c}.Sanitize(), len(setCols)+i+1)
	}

	quotedTable := pgx.Identifier{schema, table}.Sanitize()
	sql := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		quotedTable,
		joinQuoted(quotedSets),
		strings.Join(quotedWheres, " AND "),
	)

	params := make([]any, 0, len(setVals)+len(keyVals))
	params = append(params, setVals...)
	params = append(params, keyVals...)

	tag, err := d.conn.Exec(ctx, sql, params...)
	if err != nil {
		return 0, err
	}
	n := tag.RowsAffected()
	if n == 0 {
		return 0, fmt.Errorf("update row: no row found with the given key")
	}
	return n, nil
}

func mapsToColumnsAndValues(row map[string]any) (cols []string, vals []any) {
	cols = make([]string, 0, len(row))
	vals = make([]any, 0, len(row))
	for k, v := range row {
		cols = append(cols, k)
		vals = append(vals, v)
	}
	return cols, vals
}

func makePlaceholders(n int) string {
	if n == 0 {
		return ""
	}
	b := make([]byte, 0, n*4)
	for i := 1; i <= n; i++ {
		if i > 1 {
			b = append(b, ',', ' ')
		}
		b = append(b, '$')
		b = append(b, fmt.Sprintf("%d", i)...)
	}
	return string(b)
}

func joinQuoted(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for i := 1; i < len(ss); i++ {
		out += ", " + ss[i]
	}
	return out
}

// Close implements Driver.
func (d *PostgresDriver) Close() error {
	return d.conn.Close(context.Background())
}

// Ensure PostgresDriver implements Driver.
var _ Driver = (*PostgresDriver)(nil)
