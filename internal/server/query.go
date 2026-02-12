package server

import (
	"fmt"
	"regexp"
	"strings"
)

// read-only SQL: forbid keywords that modify data or schema
var forbiddenSQLWords = []string{
	"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TRUNCATE",
	"GRANT", "REVOKE", "EXEC", "EXECUTE", "MERGE", "REPLACE",
}

var (
	sqlLineComment = regexp.MustCompile(`--[^\n]*`)
	sqlBlockComment = regexp.MustCompile(`/\*[\s\S]*?\*/`)
	forbiddenWordRe = regexp.MustCompile(`(?i)\b(` + strings.Join(forbiddenSQLWords, "|") + `)\b`)
)

// ValidateReadOnlySQL returns an error if sql appears to be non–read-only (INSERT/UPDATE/DELETE/DDL etc).
// It strips line (--) and block (/* */) comments before checking. Only a simple heuristic; not a full parser.
func ValidateReadOnlySQL(sql string) error {
	cleaned := sqlLineComment.ReplaceAllString(sql, " ")
	cleaned = sqlBlockComment.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return fmt.Errorf("empty SQL after removing comments")
	}
	if loc := forbiddenWordRe.FindStringIndex(cleaned); loc != nil {
		word := strings.ToUpper(cleaned[loc[0]:loc[1]])
		return fmt.Errorf("read-only queries only: found %q", word)
	}
	return nil
}
