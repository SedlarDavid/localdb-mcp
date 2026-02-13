package db

import (
	"context"
	"testing"
)

// mockDriver implements Driver with just enough to test validatePKColumns.
type mockDriver struct {
	columns []ColumnInfo
	descErr error
}

func (m *mockDriver) Ping(context.Context) error                          { return nil }
func (m *mockDriver) ListTables(context.Context, string) ([]string, error) { return nil, nil }
func (m *mockDriver) DescribeTable(_ context.Context, _, _ string) ([]ColumnInfo, error) {
	return m.columns, m.descErr
}
func (m *mockDriver) RunReadOnlyQuery(context.Context, string, []any) ([]map[string]any, error) {
	return nil, nil
}
func (m *mockDriver) InsertRow(context.Context, string, string, map[string]any) (any, error) {
	return nil, nil
}
func (m *mockDriver) UpdateRow(context.Context, string, string, map[string]any, map[string]any) (int64, error) {
	return 0, nil
}
func (m *mockDriver) Close() error { return nil }

func TestValidatePKColumns(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		columns []ColumnInfo
		key     map[string]any
		wantErr bool
	}{
		{
			name: "single PK match",
			columns: []ColumnInfo{
				{Name: "id", IsPK: true},
				{Name: "name", IsPK: false},
			},
			key:     map[string]any{"id": 1},
			wantErr: false,
		},
		{
			name: "composite PK match",
			columns: []ColumnInfo{
				{Name: "tenant_id", IsPK: true},
				{Name: "user_id", IsPK: true},
				{Name: "name", IsPK: false},
			},
			key:     map[string]any{"tenant_id": 1, "user_id": 2},
			wantErr: false,
		},
		{
			name: "wrong key column",
			columns: []ColumnInfo{
				{Name: "id", IsPK: true},
				{Name: "name", IsPK: false},
			},
			key:     map[string]any{"name": "alice"},
			wantErr: true,
		},
		{
			name: "missing one PK column in composite key",
			columns: []ColumnInfo{
				{Name: "tenant_id", IsPK: true},
				{Name: "user_id", IsPK: true},
			},
			key:     map[string]any{"tenant_id": 1},
			wantErr: true,
		},
		{
			name: "extra column beyond PK",
			columns: []ColumnInfo{
				{Name: "id", IsPK: true},
				{Name: "name", IsPK: false},
			},
			key:     map[string]any{"id": 1, "name": "extra"},
			wantErr: true,
		},
		{
			name: "no PK on table",
			columns: []ColumnInfo{
				{Name: "a", IsPK: false},
				{Name: "b", IsPK: false},
			},
			key:     map[string]any{"a": 1},
			wantErr: true,
		},
		{
			name:    "empty table columns",
			columns: []ColumnInfo{},
			key:     map[string]any{"id": 1},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &mockDriver{columns: tt.columns}
			err := validatePKColumns(ctx, d, "public", "test_table", tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePKColumns() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
