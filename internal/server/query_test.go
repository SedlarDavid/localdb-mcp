package server

import "testing"

func TestValidateReadOnlySQL(t *testing.T) {
	tests := []struct {
		sql  string
		want bool // true = valid (no error)
	}{
		{"SELECT 1", true},
		{"SELECT * FROM users", true},
		{"WITH cte AS (SELECT 1) SELECT * FROM cte", true},
		{"select * from t", true},
		{"  SELECT 1  ", true},
		{"-- comment\nSELECT 1", true},
		{"/* comment */ SELECT 1", true},
		{"INSERT INTO t VALUES (1)", false},
		{"UPDATE t SET x = 1", false},
		{"DELETE FROM t", false},
		{"DROP TABLE t", false},
		{"CREATE TABLE t (x int)", false},
		{"ALTER TABLE t ADD c int", false},
		{"TRUNCATE t", false},
		{"SELECT 1; INSERT INTO t VALUES (1)", false},
		{"", false},
		{"   \n  -- only comment\n  ", false},
	}
	for _, tt := range tests {
		err := ValidateReadOnlySQL(tt.sql)
		ok := (err == nil)
		if ok != tt.want {
			t.Errorf("ValidateReadOnlySQL(%q): got err=%v, want valid=%v", tt.sql, err, tt.want)
		}
	}
}
