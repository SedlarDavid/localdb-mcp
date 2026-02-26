package db

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// mysqlConnInfo holds parsed MySQL DSN components for CLI tool usage.
type mysqlConnInfo struct {
	User     string
	Password string
	Host     string
	Port     string
	Database string
}

// parseMySQLDSN extracts connection components from a go-sql-driver/mysql DSN.
// Format: [user[:password]@][protocol[(address)]]/dbname[?params]
func parseMySQLDSN(dsn string) (*mysqlConnInfo, error) {
	info := &mysqlConnInfo{}

	// Split off query params
	if idx := strings.Index(dsn, "?"); idx >= 0 {
		dsn = dsn[:idx]
	}

	// Split user info from the rest at the last @
	var rest string
	if idx := strings.LastIndex(dsn, "@"); idx >= 0 {
		userPart := dsn[:idx]
		rest = dsn[idx+1:]
		if colonIdx := strings.Index(userPart, ":"); colonIdx >= 0 {
			info.User = userPart[:colonIdx]
			info.Password = userPart[colonIdx+1:]
		} else {
			info.User = userPart
		}
	} else {
		rest = dsn
	}

	// Split protocol(address) from /dbname
	var addrPart string
	if idx := strings.Index(rest, "/"); idx >= 0 {
		addrPart = rest[:idx]
		info.Database = rest[idx+1:]
	} else {
		addrPart = rest
	}

	// Extract address from protocol(address)
	if idx := strings.Index(addrPart, "("); idx >= 0 {
		endIdx := strings.Index(addrPart, ")")
		if endIdx < 0 {
			endIdx = len(addrPart)
		}
		addr := addrPart[idx+1 : endIdx]
		if colonIdx := strings.LastIndex(addr, ":"); colonIdx >= 0 {
			info.Host = addr[:colonIdx]
			info.Port = addr[colonIdx+1:]
		} else {
			info.Host = addr
		}
	}

	if info.Host == "" {
		info.Host = "localhost"
	}
	if info.Port == "" {
		info.Port = "3306"
	}
	if info.Database == "" {
		return nil, fmt.Errorf("cannot parse MySQL DSN: no database name")
	}

	return info, nil
}

func (info *mysqlConnInfo) cliArgs() []string {
	return []string{
		"--host", info.Host,
		"--port", info.Port,
		"--user", info.User,
	}
}

func (info *mysqlConnInfo) env() []string {
	if info.Password != "" {
		return []string{"MYSQL_PWD=" + info.Password}
	}
	return nil
}

// ExportDatabase dumps the MySQL database to a SQL file using mysqldump.
func (d *MySQLDriver) ExportDatabase(ctx context.Context, path string) error {
	mysqldump, err := findCLITool("mysqldump")
	if err != nil {
		return err
	}
	absPath, err := validateExportPath(path)
	if err != nil {
		return err
	}
	info, err := parseMySQLDSN(d.dsn)
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	args := info.cliArgs()
	args = append(args,
		"--result-file", absPath,
		"--single-transaction",
		"--routines",
		"--triggers",
		info.Database,
	)
	return runCLIWithEnv(ctx, info.env(), mysqldump, args...)
}

// ImportDatabase loads a SQL dump file into the MySQL database using mysql CLI.
func (d *MySQLDriver) ImportDatabase(ctx context.Context, path string) error {
	mysqlBin, err := findCLITool("mysql")
	if err != nil {
		return err
	}
	absPath, err := validateImportPath(path)
	if err != nil {
		return err
	}
	info, err := parseMySQLDSN(d.dsn)
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}

	f, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("import: open file: %w", err)
	}
	defer f.Close()

	args := info.cliArgs()
	args = append(args, info.Database)
	return runCLIWithStdin(ctx, info.env(), f, mysqlBin, args...)
}

// Ensure MySQLDriver implements Exporter.
var _ Exporter = (*MySQLDriver)(nil)
