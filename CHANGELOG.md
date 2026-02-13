# Changelog

All notable changes to this project will be documented in this file.

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
  and `go vet` on every PR to `main` and push to `main`.
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
