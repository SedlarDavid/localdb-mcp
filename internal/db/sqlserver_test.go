package db

import "testing"

func TestConvertPlaceholdersToMSSQL(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"SELECT * FROM t WHERE a = $1", "SELECT * FROM t WHERE a = @p1"},
		{"$1 AND $2", "@p1 AND @p2"},
		{"$10", "@p10"},
		{"no placeholders", "no placeholders"},
	}
	for _, tt := range tests {
		got := convertPlaceholdersToMSSQL(tt.in)
		if got != tt.want {
			t.Errorf("convertPlaceholdersToMSSQL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
