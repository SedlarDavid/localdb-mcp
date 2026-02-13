package db

import (
	"context"
	"testing"
)

func newTestSQLiteDriver(t *testing.T) *SQLiteDriver {
	t.Helper()
	d, err := NewSQLiteDriver(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteDriver: %v", err)
	}
	// Create a test table.
	_, err = d.db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		email TEXT
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	return d
}

func TestSQLite_Ping(t *testing.T) {
	d := newTestSQLiteDriver(t)
	defer d.Close()
	if err := d.Ping(context.Background()); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

func TestSQLite_ListTables(t *testing.T) {
	d := newTestSQLiteDriver(t)
	defer d.Close()
	tables, err := d.ListTables(context.Background(), "")
	if err != nil {
		t.Fatalf("ListTables: %v", err)
	}
	// Should contain "users" (and possibly sqlite_sequence, but we filter sqlite_* out).
	found := false
	for _, name := range tables {
		if name == "users" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'users' in tables, got %v", tables)
	}
}

func TestSQLite_DescribeTable(t *testing.T) {
	d := newTestSQLiteDriver(t)
	defer d.Close()
	cols, err := d.DescribeTable(context.Background(), "", "users")
	if err != nil {
		t.Fatalf("DescribeTable: %v", err)
	}
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(cols))
	}
	// id should be PK
	if cols[0].Name != "id" || !cols[0].IsPK {
		t.Errorf("expected id as PK, got %+v", cols[0])
	}
	// name should be non-nullable
	if cols[1].Name != "name" || cols[1].Nullable {
		t.Errorf("expected name as NOT NULL, got %+v", cols[1])
	}
}

func TestSQLite_InsertRow(t *testing.T) {
	d := newTestSQLiteDriver(t)
	defer d.Close()
	ctx := context.Background()

	id, err := d.InsertRow(ctx, "", "users", map[string]any{
		"name":  "Alice",
		"email": "alice@test.com",
	})
	if err != nil {
		t.Fatalf("InsertRow: %v", err)
	}
	if id == nil {
		t.Fatal("expected non-nil id")
	}
	if id.(int64) != 1 {
		t.Errorf("expected id=1, got %v", id)
	}
}

func TestSQLite_UpdateRow(t *testing.T) {
	d := newTestSQLiteDriver(t)
	defer d.Close()
	ctx := context.Background()

	// Insert a row first.
	_, err := d.InsertRow(ctx, "", "users", map[string]any{
		"name":  "Alice",
		"email": "alice@test.com",
	})
	if err != nil {
		t.Fatalf("InsertRow: %v", err)
	}

	// Update it.
	n, err := d.UpdateRow(ctx, "", "users",
		map[string]any{"id": int64(1)},
		map[string]any{"email": "new@test.com"},
	)
	if err != nil {
		t.Fatalf("UpdateRow: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 row affected, got %d", n)
	}

	// Verify the update via query.
	rows, err := d.RunReadOnlyQuery(ctx, "SELECT email FROM users WHERE id = ?1", []any{int64(1)})
	if err != nil {
		t.Fatalf("RunReadOnlyQuery: %v", err)
	}
	if len(rows) != 1 || rows[0]["email"] != "new@test.com" {
		t.Errorf("expected email=new@test.com, got %v", rows)
	}
}

func TestSQLite_UpdateRow_wrongKey(t *testing.T) {
	d := newTestSQLiteDriver(t)
	defer d.Close()
	ctx := context.Background()

	// Try to update with a non-PK column as key.
	_, err := d.UpdateRow(ctx, "", "users",
		map[string]any{"name": "Alice"},
		map[string]any{"email": "x"},
	)
	if err == nil {
		t.Fatal("expected error for non-PK key column")
	}
}

func TestSQLite_UpdateRow_noop(t *testing.T) {
	d := newTestSQLiteDriver(t)
	defer d.Close()
	ctx := context.Background()

	// Insert a row.
	_, err := d.InsertRow(ctx, "", "users", map[string]any{
		"name":  "Alice",
		"email": "alice@test.com",
	})
	if err != nil {
		t.Fatalf("InsertRow: %v", err)
	}

	// Update with the same values — SQLite reports matched rows (not
	// changed rows), so this should succeed with 1 row affected.
	n, err := d.UpdateRow(ctx, "", "users",
		map[string]any{"id": int64(1)},
		map[string]any{"email": "alice@test.com"},
	)
	if err != nil {
		t.Fatalf("UpdateRow (noop) should not error, got: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 row affected for noop update, got %d", n)
	}
}

func TestSQLite_UpdateRow_notFound(t *testing.T) {
	d := newTestSQLiteDriver(t)
	defer d.Close()
	ctx := context.Background()

	_, err := d.UpdateRow(ctx, "", "users",
		map[string]any{"id": int64(999)},
		map[string]any{"email": "x"},
	)
	if err == nil {
		t.Fatal("expected error for non-existent row")
	}
}

func TestSQLite_RunReadOnlyQuery(t *testing.T) {
	d := newTestSQLiteDriver(t)
	defer d.Close()
	ctx := context.Background()

	_, _ = d.InsertRow(ctx, "", "users", map[string]any{"name": "Bob", "email": "bob@test.com"})

	rows, err := d.RunReadOnlyQuery(ctx, "SELECT name, email FROM users WHERE name = ?1", []any{"Bob"})
	if err != nil {
		t.Fatalf("RunReadOnlyQuery: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["name"] != "Bob" {
		t.Errorf("expected name=Bob, got %v", rows[0]["name"])
	}
}

func TestConvertPlaceholdersToSQLite(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"SELECT * FROM t WHERE a = $1", "SELECT * FROM t WHERE a = ?1"},
		{"$1 AND $2", "?1 AND ?2"},
		{"$10", "?10"},
		{"no placeholders", "no placeholders"},
	}
	for _, tt := range tests {
		got := convertPlaceholdersToSQLite(tt.in)
		if got != tt.want {
			t.Errorf("convertPlaceholdersToSQLite(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
