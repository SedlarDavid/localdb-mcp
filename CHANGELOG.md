# Changelog

All notable changes to this project will be documented in this file.

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
