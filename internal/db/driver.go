// Package db provides database driver abstraction and connection management
// for PostgreSQL and SQL Server.
package db

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

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
	// UpdateRow updates a single row identified by its primary key columns.
	// key holds PK column→value pairs used in WHERE; set holds column→value pairs for SET.
	// The implementation must verify that key columns match the table's actual PK
	// and return an error if they don't or if no row is found.
	UpdateRow(ctx context.Context, schema, table string, key map[string]any, set map[string]any) (rowsAffected int64, err error)
	// Close releases the connection. Caller should call once when done.
	Close() error
}

// validatePKColumns fetches the real primary key columns of a table via
// DescribeTable and verifies that the caller-provided key map matches them
// exactly (same column names, no extra, no missing).
func validatePKColumns(ctx context.Context, d Driver, schema, table string, key map[string]any) error {
	cols, err := d.DescribeTable(ctx, schema, table)
	if err != nil {
		return fmt.Errorf("update row: failed to describe table: %w", err)
	}

	var pkCols []string
	for _, c := range cols {
		if c.IsPK {
			pkCols = append(pkCols, c.Name)
		}
	}
	if len(pkCols) == 0 {
		return fmt.Errorf("update row: table %q has no primary key; update_test_row requires one", table)
	}

	// Collect provided key column names.
	provided := make([]string, 0, len(key))
	for k := range key {
		provided = append(provided, k)
	}

	// Sort both for comparison.
	sort.Strings(pkCols)
	sort.Strings(provided)

	if strings.Join(provided, ",") != strings.Join(pkCols, ",") {
		return fmt.Errorf(
			"update row: key columns {%s} do not match primary key {%s}",
			strings.Join(provided, ", "),
			strings.Join(pkCols, ", "),
		)
	}
	return nil
}

// ColumnInfo describes one column for describe_table.
type ColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	IsPK     bool   `json:"is_pk"`
}
