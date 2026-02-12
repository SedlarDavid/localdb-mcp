# localdb-mcp

**v1.0.0** — Local [MCP](https://modelcontextprotocol.io/) server that gives AI agents and LLMs (e.g. Cursor, Claude Code, other MCP clients) access to your databases **without exposing credentials** to the model. The server runs on your machine, reads connection details from env or config, and exposes tools: list tables, describe table, read-only query, insert test row. Agents call tools by name; connection strings stay in the server process.

## Use with agents / LLMs

- **Cursor, Claude Code, or any MCP client:** Add this server in your MCP settings. The client runs the server (e.g. `./localdb-mcp` or `go run ./cmd/server`). The agent sees only tool names and JSON input/output; it never sees connection strings or credentials.
- **Workflow:** You configure Postgres or SQL Server (e.g. in Docker) and point the server at them via env or a config file. When you ask the agent to “add test data” or “show me the schema,” it calls tools like `list_tables`, `describe_table`, `run_query`, or `insert_test_row`. The agent does not need to know host, port, user, or password.
- **Typical use:** Local or dev databases only — generating test data, inspecting schema, running read-only queries. Not for production (see Disclaimer below).

## Why

- Credentials only in env or `~/.localdb-mcp/config.yaml` — never in the IDE/MCP config or tool responses.
- Fixed tool set so the agent doesn’t need host/port/user/password.
- Supports PostgreSQL and SQL Server (e.g. in Docker) as named connections `postgres` and `sqlserver`.

## Requirements

- Go 1.25+
- PostgreSQL and/or SQL Server reachable (e.g. Docker) for DB tools.

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

3. **Add to your MCP client** — In Cursor (or your client) MCP settings, add a server with command `./localdb-mcp` or `go run ./cmd/server` (from repo root). Do not put credentials in the client config; the server reads env/file.

## Tools

| Tool | Description |
|------|-------------|
| `ping` | Health check → `{"message":"pong"}` |
| `list_connections` | Configured connection IDs and types (no credentials) |
| `list_tables` | `connection_id`, optional `schema` → table names |
| `describe_table` | `connection_id`, `table`, optional `schema` → columns (name, type, nullable, is_pk) |
| `run_query` (read-only) | `connection_id`, `sql`, optional `params` → rows. Rejects INSERT/UPDATE/DELETE/DDL. |
| `insert_test_row` | `connection_id`, `table`, `row`, optional `schema`, `return_id` → optional `inserted_id` |

## Safety

Read-only by default; `run_query` allows only SELECT (and read-only SQL). Writes only via `insert_test_row`. No DDL. Credentials are never included in tool results or logs.

---

## Disclaimer

**This software is for development and testing only.**

- **Do not use it with production databases or production secrets.** Use only with local or dedicated dev/test databases and credentials you can afford to expose on your machine.
- **No warranty.** The software is provided “as is.” The author and contributors are not responsible for any damage, data loss, or misuse arising from the use of this software, including (but not limited to) misconfiguration, credential leakage, or use against production systems.
- **You are responsible** for how you configure and run the server and for keeping credentials out of production use.

---

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

- `cmd/server` — MCP server entrypoint (stdio)
- `cmd/mcpclient` — CLI to call any tool (for testing)
- `internal/config` — env + optional `.env` (cwd) and `~/.localdb-mcp/config.yaml`
- `internal/server` — MCP server and tool registration
- `internal/db` — Driver interface, Postgres/SQL Server implementations, connection manager

## Contributing

Contributions are welcome. Please open an issue to discuss larger changes, or send a pull request for bug fixes and small improvements. By contributing, you agree that your submissions will be licensed under the same [MIT](LICENSE) terms as this project.

## License

[MIT](LICENSE). Use at your own risk; see Disclaimer above.
