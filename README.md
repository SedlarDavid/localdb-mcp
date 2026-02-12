# localdb-mcp

Local MCP server that gives AI agents (e.g. Cursor) access to your databases **without seeing credentials**. The server runs on your machine, reads connection details from env or config, and exposes tools (list tables, describe table, read-only query, insert test row). Agents call tools by name; connection strings stay in the server process.

## Why

- Credentials only in env or `~/.localdb-mcp/config.yaml` — never in Cursor config or tool responses.
- Fixed tool set so the agent doesn’t need host/port/user/password.
- Supports PostgreSQL and SQL Server (e.g. in Docker) as named connections `postgres` and `sqlserver`.

## Requirements

- Go 1.25+
- (Later) PostgreSQL and/or SQL Server reachable (e.g. Docker).

## Quick start

1. **Build and run**

   ```bash
   go build -o localdb-mcp ./cmd/server
   ./localdb-mcp
   ```

   Or: `go run ./cmd/server`

2. **Configure** (optional for `ping` and `list_connections`; needed for `list_tables`, `describe_table`, etc.)

   - Env or **.env**: see **.env.example** for `MCP_DB_POSTGRES_URI` and `MCP_DB_SQLSERVER_URI`. The server loads `.env` from its working directory if present; otherwise export in your shell.
   - Optional file: `~/.localdb-mcp/config.yaml` with `connections: { postgres: "uri", sqlserver: "uri" }`. Env overrides file.

3. **Cursor** — In MCP settings, add a server with command `./localdb-mcp` or `go run ./cmd/server` (from repo root). No credentials in Cursor; server reads env/file.

## Tools

| Tool | Status |
|------|--------|
| `ping` | ✅ Health check → `{"message":"pong"}` |
| `list_connections` | ✅ Returns `{"connections":[{"id":"postgres","type":"postgres"},...]}` (no credentials) |
| `list_tables` | ✅ `connection_id`, optional `schema` → `{"tables":["..."]}` |
| `describe_table` | ✅ `connection_id`, `table`, optional `schema` → columns (name, type, nullable, is_pk) |
| `run_query` (read-only) | ✅ `connection_id`, `sql`, optional `params` → `{"rows":[...]}`. Rejects INSERT/UPDATE/DELETE/DDL. |
| `insert_test_row` | ✅ `connection_id`, `table`, `row`, optional `schema`, `return_id` → optional `inserted_id` |

## Safety

Read-only by default; `run_query` will allow only SELECT. Writes only via `insert_test_row`. No DDL. Credentials never in tool results or logs.

## Testing

```bash
go test ./...
go run ./cmd/mcpclient ping
go run ./cmd/mcpclient list_connections
go run ./cmd/mcpclient list_tables '{"connection_id":"postgres"}'
go run ./cmd/mcpclient describe_table '{"connection_id":"postgres","table":"users"}'
go run ./cmd/mcpclient run_query '{"connection_id":"postgres","sql":"SELECT 1"}'
go run ./cmd/mcpclient insert_test_row '{"connection_id":"postgres","table":"users","row":{"name":"Test"}}'
```

## Layout

- `cmd/server` — entrypoint (stdio)
- `cmd/mcpclient` — CLI to call any tool (spawns server, passes tool name + optional JSON args)
- `internal/config` — env + optional `.env` (cwd) and `~/.localdb-mcp/config.yaml`
- `internal/server` — MCP server and tool registration
- `internal/db` — Driver interface, Postgres/SQL Server implementations, connection manager

## License

See repository license.
