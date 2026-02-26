# Changelog

All notable changes to this project will be documented in this file.

## [1.2.0] - 2026-02-26

### Added

- **`export_database` tool.** Export a database to a SQL dump file using
  engine-native CLI tools. PostgreSQL uses `pg_dump`, MySQL uses `mysqldump`,
  SQLite uses `sqlite3 .dump`, SQL Server generates SQL via pure Go queries.
  On macOS, automatically discovers Homebrew-installed versioned PostgreSQL
  binaries (e.g. `postgresql@18`).
- **`import_database` tool.** Import a SQL dump file into a database using
  engine-native CLI tools. PostgreSQL uses `psql`, MySQL uses `mysql` CLI,
  SQLite uses `sqlite3`, SQL Server uses `sqlcmd`. Requires explicit
  `confirm_destructive=true` parameter since this is a destructive operation.
- **`Exporter` interface.** New optional interface in the `db` package,
  separate from `Driver`, keeping the existing contract clean.
- Shared CLI utilities for path validation, tool discovery, and safe external
  command execution with credential-safe error reporting.

### Changed

- All driver structs now store their connection URI/DSN for CLI tool usage.
- README updated with new tools and safety documentation.

## [1.1.0] - 2026-02-13

### Added

- **`update_test_row` tool.** Safely update a single row by primary key.
  Structurally enforces PK-only targeting — validates that the provided key
  columns match the table's actual primary key to prevent mass updates.
  Returns `rows_affected` count.
- **SQLite support.** New `sqlite` connection type using modernc.org/sqlite
  (pure Go, no CGO). Configure via `MCP_DB_SQLITE_URI` env var. Supports
  file paths, URIs, and `:memory:` for in-memory databases.
- **MySQL support.** New `mysql` connection type using go-sql-driver/mysql.
  Configure via `MCP_DB_MYSQL_URI` env var with DSN format
  (`user:pass@tcp(host:port)/db`).
- **CI pipeline.** GitHub Actions workflow runs build, test (with `-race`),
  `go vet`, and golangci-lint on every PR to `main` and push to `main`.
- **Static analysis.** golangci-lint v2 config (`.golangci.yml`) with
  standard linters plus bodyclose, nilerr, errname, errorlint, copyloopvar,
  gocritic, and misspell.
- Comprehensive test suite for `validatePKColumns` (7 cases) and SQLite
  integration tests (9 cases covering all Driver methods).

### Changed

- `Driver` interface now includes `UpdateRow` method.
- `list_connections` description updated to reflect all supported types.
- README updated with new tools, database types, configuration, and CI section.

## [1.0.1] - 2026-02-13

### Fixed

- **MCP tool discovery in Cursor and other IDE agents.** The previous MCP SDK
  (modelcontextprotocol/go-sdk) emitted a non-standard outputSchema field in
  tool definitions, causing clients like Cursor to report
  "No tools, prompts, or resources" despite a successful connection. Migrated to
  mark3labs/mcp-go which produces spec-compliant tool schemas.

### Changed

- Replaced modelcontextprotocol/go-sdk with mark3labs/mcp-go for both server
  and client.
- Rewrote tool registration to use mcp-go functional options API
  (mcp.NewTool, mcp.WithString, etc.).
- Rewrote cmd/mcpclient test client to use mcp-go stdio client.
- Updated internal/server/server_test.go to use mcp-go in-process client.
- Added optional debug logging to file (/tmp/localdb-mcp.log) when MCP_DEBUG
  env var is set.

## [1.0.0] - 2026-02-12

### Added

- Initial release.
- MCP server with stdio transport for use with Cursor, Claude Desktop, and
  other MCP clients.
- Tools: ping, list_connections, list_tables, describe_table,
  run_query (read-only), insert_test_row.
- PostgreSQL and SQL Server support via named connections.
- Configuration via environment variables (MCP_DB_POSTGRES_URI,
  MCP_DB_SQLSERVER_URI), .env file, or ~/.localdb-mcp/config.yaml.
- Read-only SQL validation (rejects INSERT/UPDATE/DELETE/DDL).
- Credentials never exposed in tool responses or logs.
- cmd/mcpclient CLI for testing tool calls.

[1.2.0]: https://github.com/SedlarDavid/localdb-mcp/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/SedlarDavid/localdb-mcp/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/SedlarDavid/localdb-mcp/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/SedlarDavid/localdb-mcp/releases/tag/v1.0.0
