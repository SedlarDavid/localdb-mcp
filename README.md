# localdb-mcp

[![CI](https://github.com/SedlarDavid/localdb-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/SedlarDavid/localdb-mcp/actions/workflows/ci.yml)
[![license](https://img.shields.io/github/license/SedlarDavid/localdb-mcp)](LICENSE)
[![release](https://img.shields.io/github/v/release/SedlarDavid/localdb-mcp)](https://github.com/SedlarDavid/localdb-mcp/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/SedlarDavid/localdb-mcp)](https://goreportcard.com/report/github.com/SedlarDavid/localdb-mcp)

**v1.1.0** — Local [MCP](https://modelcontextprotocol.io/) server that gives AI agents and LLMs (e.g. Cursor, Claude Code, other MCP clients) access to your databases **without exposing credentials** to the model. The server runs on your machine, reads connection details from env or config, and exposes a fixed set of tools (see [Tools](#tools) below). Agents call tools by name; connection strings stay in the server process.

## Use with agents / LLMs

- **Cursor, Claude Code, or any MCP client:** Add this server in your MCP settings. The client runs the server (e.g. `./localdb-mcp` or `go run ./cmd/server`). The agent sees only tool names and JSON input/output; it never sees connection strings or credentials.
- **Workflow:** You configure your databases (Postgres, SQL Server, MySQL via Docker, or a local SQLite file) and point the server at them via env or config. When you ask the agent to “add test data” or “update a timestamp,” it calls tools like `list_tables`, `describe_table`, `run_query`, `insert_test_row`, or `update_test_row`. The agent does not need to know host, port, user, or password.
- **Typical use:** Local or dev databases only — generating test data, inspecting schema, running read-only queries. Not for production (see Disclaimer below).

## Why

- Credentials only in env or `~/.localdb-mcp/config.yaml` — never in the IDE/MCP config or tool responses.
- Fixed tool set so the agent doesn’t need host/port/user/password.
- Supports PostgreSQL, SQL Server, SQLite, and MySQL as named connections `postgres`, `sqlserver`, `sqlite`, and `mysql`.

## Requirements

- Go 1.25+
- PostgreSQL, SQL Server, MySQL reachable (e.g. Docker) for DB tools, or a SQLite file/`:memory:`.

## Quick start

1. **Build and run**

   ```bash
   go build -o localdb-mcp ./cmd/server
   ./localdb-mcp
   ```

   Or: `go run ./cmd/server`

2. **Configure** (optional for `ping` and `list_connections`; needed for `list_tables`, `describe_table`, etc.)

   - Env or **.env**: see **.env.example** for `MCP_DB_POSTGRES_URI`, `MCP_DB_SQLSERVER_URI`, `MCP_DB_SQLITE_URI`, and `MCP_DB_MYSQL_URI`. The server loads `.env` from its working directory if present; otherwise export in your shell.
   - Optional file: `~/.localdb-mcp/config.yaml` with `connections: { postgres: "uri", sqlserver: "uri", sqlite: "/path/to/db.sqlite", mysql: "user:pass@tcp(host:3306)/db" }`. Env overrides file.

3. **Add to your MCP client** — See below for configuration examples.

## Client Configuration

### Cursor

Go to **Settings** > **Features** > **MCP Servers** > **+ Add new MCP server**:

- **Name:** `localdb` (or any name)
- **Type:** `command`
- **Command:** Absolute path to your binary (e.g. `/Users/me/dev/localdb-mcp/localdb-mcp`) or `go run ...`
- **Args:** (leave empty if using binary, or use `run /path/to/cmd/server` if using `go`)

Or manually edit `.cursor/mcp.json` (if project-specific):

```json
{
  "mcpServers": {
    "localdb": {
      "command": "/Users/me/dev/localdb-mcp/localdb-mcp",
      "args": [],
      "env": {
        "MCP_DB_POSTGRES_URI": "postgres://user:pass@localhost:5432/db",
        "MCP_DB_SQLSERVER_URI": "sqlserver://sa:Pass@localhost:1433"
      }
    }
  }
}
```

> **Note:** Cursor can also read the `.env` file in the server's working directory if you run it from there, but setting environment variables explicitly in the config is often more reliable.

### Claude Desktop

Edit your configuration file:
- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "localdb": {
      "command": "/Users/me/dev/localdb-mcp/localdb-mcp",
      "env": {
         "MCP_DB_POSTGRES_URI": "postgres://user:pass@localhost:5432/db"
      }
    }
  }
}
```

### VS Code (Generic MCP Extension)

If using an MCP extension in VS Code, you typically add it to `.vscode/settings.json` or the extension's specific config:

```json
"mcp.servers": {
  "localdb": {
    "command": "/path/to/localdb-mcp",
    "transport": "stdio"
  }
}
```

## Tools

| Tool | Description |
|------|-------------|
| `ping` | Health check → `{"message":"pong"}` |
| `list_connections` | Configured connection IDs and types (no credentials) |
| `list_tables` | `connection_id`, optional `schema` → table names |
| `describe_table` | `connection_id`, `table`, optional `schema` → columns (name, type, nullable, is_pk) |
| `run_query` (read-only) | `connection_id`, `sql`, optional `params` → rows. Rejects INSERT/UPDATE/DELETE/DDL. |
| `insert_test_row` | `connection_id`, `table`, `row`, optional `schema`, `return_id` → optional `inserted_id` |
| `update_test_row` | `connection_id`, `table`, `key` (PK), `set` (values), optional `schema` → `rows_affected` |

## Safety

Read-only by default; `run_query` allows only SELECT (and read-only SQL). Writes only via `insert_test_row` and `update_test_row`. `update_test_row` enforces primary-key-only targeting — it validates that the `key` columns match the table's actual PK to prevent mass updates. No DDL. Credentials are never included in tool results or logs.

---

## Disclaimer

**This software is for development and testing only.**

- **Do not use it with production databases or production secrets.** Use only with local or dedicated dev/test databases and credentials you can afford to expose on your machine.
- **No warranty.** The software is provided “as is.” The author and contributors are not responsible for any damage, data loss, or misuse arising from the use of this software, including (but not limited to) misconfiguration, credential leakage, or use against production systems.
- **You are responsible** for how you configure and run the server and for keeping credentials out of production use.

---

## CI

GitHub Actions runs on every PR to `main` and on push to `main`: build, test (`-race`), `go vet`, and [golangci-lint](https://golangci-lint.run/) static analysis. See [`.github/workflows/ci.yml`](.github/workflows/ci.yml) and [`.golangci.yml`](.golangci.yml).

## Testing

```bash
go test ./...
go run ./cmd/mcpclient ping
go run ./cmd/mcpclient list_connections
go run ./cmd/mcpclient list_tables '{"connection_id":"postgres"}'
go run ./cmd/mcpclient describe_table '{"connection_id":"postgres","table":"users"}'
go run ./cmd/mcpclient run_query '{"connection_id":"postgres","sql":"SELECT 1"}'
go run ./cmd/mcpclient insert_test_row '{"connection_id":"postgres","table":"users","row":{"name":"Test"}}'
go run ./cmd/mcpclient update_test_row '{"connection_id":"postgres","table":"users","key":{"id":1},"set":{"name":"Updated"}}'
```

## Layout

- `cmd/server` — MCP server entrypoint (stdio)
- `cmd/mcpclient` — CLI to call any tool (for testing)
- `internal/config` — env + optional `.env` (cwd) and `~/.localdb-mcp/config.yaml`
- `internal/server` — MCP server and tool registration
- `internal/db` — Driver interface, Postgres/SQL Server/SQLite/MySQL implementations, connection manager

## Contributing

Contributions are welcome. Please open an issue to discuss larger changes, or send a pull request for bug fixes and small improvements. By contributing, you agree that your submissions will be licensed under the same [MIT](LICENSE) terms as this project.

## License

[MIT](LICENSE). Use at your own risk; see Disclaimer above.
