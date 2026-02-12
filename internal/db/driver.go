// Package db provides database driver abstraction and connection management
// for PostgreSQL and SQL Server.
package db

import "context"

// Driver is the interface for database operations used by MCP tools.
// Implementations are backend-specific (Postgres, SQL Server).
type Driver interface {
	// Ping verifies the connection is alive.
	Ping(ctx context.Context) error
	// ListTables returns table names in the given schema (e.g. "public").
	ListTables(ctx context.Context, schema string) ([]string, error)
	// DescribeTable returns column metadata for the given schema and table.
	DescribeTable(ctx context.Context, schema, table string) ([]ColumnInfo, error)
	// RunReadOnlyQuery runs a read-only SQL statement (caller must validate).
	// Params are positional ($1, $2 in Postgres; @p1, @p2 in SQL Server).
	// Returns rows as slice of column-name -> value maps.
	RunReadOnlyQuery(ctx context.Context, sql string, params []any) ([]map[string]any, error)
	// InsertRow inserts a single row; row keys are column names, values are column values.
	// Returns the generated ID if the table has a single identity/serial column, else nil.
	InsertRow(ctx context.Context, schema, table string, row map[string]any) (insertedID any, err error)
	// Close releases the connection. Caller should call once when done.
	Close() error
}

// ColumnInfo describes one column for describe_table.
type ColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	IsPK     bool   `json:"is_pk"`
}
