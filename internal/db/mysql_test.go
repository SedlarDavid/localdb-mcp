package db

import "testing"

func TestConvertPlaceholdersToMySQL(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"SELECT * FROM t WHERE a = $1", "SELECT * FROM t WHERE a = ?"},
		{"$1 AND $2", "? AND ?"},
		{"$10", "?"},
		{"no placeholders", "no placeholders"},
	}
	for _, tt := range tests {
		got := convertPlaceholdersToMySQL(tt.in)
		if got != tt.want {
			t.Errorf("convertPlaceholdersToMySQL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMakeMySQLPlaceholders(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, ""},
		{1, "?"},
		{3, "?, ?, ?"},
	}
	for _, tt := range tests {
		got := makeMySQLPlaceholders(tt.n)
		if got != tt.want {
			t.Errorf("makeMySQLPlaceholders(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestQuoteMySQLTable(t *testing.T) {
	tests := []struct {
		schema, table string
		want          string
	}{
		{"", "users", "`users`"},
		{"mydb", "users", "`mydb`.`users`"},
		{"", "user`name", "`user``name`"},
	}
	for _, tt := range tests {
		got := quoteMySQLTable(tt.schema, tt.table)
		if got != tt.want {
			t.Errorf("quoteMySQLTable(%q, %q) = %q, want %q", tt.schema, tt.table, got, tt.want)
		}
	}
}
