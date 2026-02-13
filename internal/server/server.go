// Package server builds the MCP server and registers tools.
package server

import (
	"context"

	"github.com/SedlarDavid/localdb-mcp/internal/config"
	"github.com/SedlarDavid/localdb-mcp/internal/db"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ServerName    = "localdb-mcp"
	ServerVersion = "1.1.0"
)

// Register registers tools to the MCP server.
func Register(s *server.MCPServer, cfg *config.Config) {
	var mgr *db.Manager
	if cfg != nil {
		mgr = db.NewManager(cfg)
	}

	// Ping
	s.AddTool(mcp.NewTool("ping",
		mcp.WithDescription("Simple health check. Returns pong."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultJSON(PingOutput{Message: "pong"})
	})

	// List Connections
	s.AddTool(mcp.NewTool("list_connections",
		mcp.WithDescription("List configured database connection IDs and their types (postgres, sqlserver, sqlite, mysql). No credentials in response."),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out := ListConnectionsOutput{Connections: nil}
		if cfg != nil {
			out.Connections = cfg.ConnectionInfos()
		}
		return mcp.NewToolResultJSON(out)
	})

	if mgr != nil {
		// List Tables
		s.AddTool(mcp.NewTool("list_tables",
			mcp.WithDescription("List table names in a given connection and optional schema."),
			mcp.WithString("connection_id", mcp.Required(), mcp.Description("Connection ID")),
			mcp.WithString("schema", mcp.Description("Schema (optional)")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args, ok := request.Params.Arguments.(map[string]any)
			if !ok {
				return mcp.NewToolResultError("invalid arguments"), nil
			}
			
			connID, ok := args["connection_id"].(string)
			if !ok {
				return mcp.NewToolResultError("connection_id is required"), nil
			}
			schema, _ := args["schema"].(string)

			driver, err := mgr.Driver(ctx, connID)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			tables, err := driver.ListTables(ctx, schema)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			return mcp.NewToolResultJSON(ListTablesOutput{Tables: tables})
		})

		// Describe Table
		s.AddTool(mcp.NewTool("describe_table",
			mcp.WithDescription("Describe columns of a table (name, type, nullable, primary key)."),
			mcp.WithString("connection_id", mcp.Required(), mcp.Description("Connection ID")),
			mcp.WithString("table", mcp.Required(), mcp.Description("Table name")),
			mcp.WithString("schema", mcp.Description("Schema (optional)")),
		), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args, ok := request.Params.Arguments.(map[string]any)
			if !ok {
				return mcp.NewToolResultError("invalid arguments"), nil
			}

			connID, ok := args["connection_id"].(string)
			if !ok {
				return mcp.NewToolResultError("connection_id is required"), nil
			}
			table, ok := args["table"].(string)
			if !ok {
				return mcp.NewToolResultError("table is required"), nil
			}
			schema, _ := args["schema"].(string)

			driver, err := mgr.Driver(ctx, connID)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cols, err := driver.DescribeTable(ctx, schema, table)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			return mcp.NewToolResultJSON(DescribeTableOutput{Columns: cols})
		})

		// Run Query
		runQueryTool := mcp.NewTool("run_query",
			mcp.WithDescription("Run a read-only SQL query (SELECT only). Rejects INSERT/UPDATE/DELETE/DDL. Params are positional."),
			mcp.WithString("connection_id", mcp.Required(), mcp.Description("Connection ID")),
			mcp.WithString("sql", mcp.Required(), mcp.Description("SQL query")),
		)
		// Manually add params array to schema
		runQueryTool.InputSchema.Properties["params"] = map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": []string{"string", "number", "boolean", "null"}, 
			},
			"description": "Positional parameters for the query",
		}

		s.AddTool(runQueryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args, ok := request.Params.Arguments.(map[string]any)
			if !ok {
				return mcp.NewToolResultError("invalid arguments"), nil
			}

			connID, ok := args["connection_id"].(string)
			if !ok {
				return mcp.NewToolResultError("connection_id is required"), nil
			}
			sql, ok := args["sql"].(string)
			if !ok {
				return mcp.NewToolResultError("sql is required"), nil
			}

			var params []any
			if p, ok := args["params"]; ok {
				if pList, ok := p.([]any); ok {
					params = pList
				}
			}

			if err := ValidateReadOnlySQL(sql); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			driver, err := mgr.Driver(ctx, connID)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			rows, err := driver.RunReadOnlyQuery(ctx, sql, params)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			return mcp.NewToolResultJSON(RunQueryOutput{Rows: rows})
		})

		// Insert Test Row
		insertRowTool := mcp.NewTool("insert_test_row",
			mcp.WithDescription("Insert a single test row. Optionally return generated ID (e.g. serial/identity)."),
			mcp.WithString("connection_id", mcp.Required(), mcp.Description("Connection ID")),
			mcp.WithString("table", mcp.Required(), mcp.Description("Table name")),
			mcp.WithBoolean("return_id", mcp.Description("Return generated ID")),
			mcp.WithString("schema", mcp.Description("Schema (optional)")),
		)
		insertRowTool.InputSchema.Properties["row"] = map[string]any{
			"type":                 "object",
			"additionalProperties": true,
			"description":          "Column names and values to insert",
		}
		insertRowTool.InputSchema.Required = append(insertRowTool.InputSchema.Required, "row")

		s.AddTool(insertRowTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args, ok := request.Params.Arguments.(map[string]any)
			if !ok {
				return mcp.NewToolResultError("invalid arguments"), nil
			}

			connID, ok := args["connection_id"].(string)
			if !ok {
				return mcp.NewToolResultError("connection_id is required"), nil
			}
			table, ok := args["table"].(string)
			if !ok {
				return mcp.NewToolResultError("table is required"), nil
			}
			returnID, _ := args["return_id"].(bool)
			schema, _ := args["schema"].(string)

			rowMap, ok := args["row"].(map[string]any)
			if !ok || len(rowMap) == 0 {
				return mcp.NewToolResultError("row is required and must be an object"), nil
			}

			driver, err := mgr.Driver(ctx, connID)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			id, err := driver.InsertRow(ctx, schema, table, rowMap)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			out := InsertTestRowOutput{}
			if returnID && id != nil {
				out.InsertedID = id
			}
			return mcp.NewToolResultJSON(out)
		})

		// Update Test Row
		updateRowTool := mcp.NewTool("update_test_row",
			mcp.WithDescription("Update a single row identified by its primary key. Safely enforces PK-only targeting to prevent mass updates."),
			mcp.WithString("connection_id", mcp.Required(), mcp.Description("Connection ID")),
			mcp.WithString("table", mcp.Required(), mcp.Description("Table name")),
			mcp.WithString("schema", mcp.Description("Schema (optional)")),
		)
		updateRowTool.InputSchema.Properties["key"] = map[string]any{
			"type":                 "object",
			"additionalProperties": true,
			"description":          "Primary key column(s) and their values to identify the row",
		}
		updateRowTool.InputSchema.Properties["set"] = map[string]any{
			"type":                 "object",
			"additionalProperties": true,
			"description":          "Column names and new values to update",
		}
		updateRowTool.InputSchema.Required = append(updateRowTool.InputSchema.Required, "key", "set")

		s.AddTool(updateRowTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args, ok := request.Params.Arguments.(map[string]any)
			if !ok {
				return mcp.NewToolResultError("invalid arguments"), nil
			}

			connID, ok := args["connection_id"].(string)
			if !ok {
				return mcp.NewToolResultError("connection_id is required"), nil
			}
			table, ok := args["table"].(string)
			if !ok {
				return mcp.NewToolResultError("table is required"), nil
			}
			schema, _ := args["schema"].(string)

			keyMap, ok := args["key"].(map[string]any)
			if !ok || len(keyMap) == 0 {
				return mcp.NewToolResultError("key is required and must be an object with PK column(s)"), nil
			}
			setMap, ok := args["set"].(map[string]any)
			if !ok || len(setMap) == 0 {
				return mcp.NewToolResultError("set is required and must be an object with column(s) to update"), nil
			}

			driver, err := mgr.Driver(ctx, connID)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			n, err := driver.UpdateRow(ctx, schema, table, keyMap, setMap)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			return mcp.NewToolResultJSON(UpdateTestRowOutput{RowsAffected: n})
		})
	}
}

// PingOutput is the structured result of the ping tool.
type PingOutput struct {
	Message string `json:"message"`
}

// ListConnectionsOutput is the result of list_connections.
type ListConnectionsOutput struct {
	Connections []config.ConnectionInfo `json:"connections"`
}

// ListTablesOutput is the result of list_tables.
type ListTablesOutput struct {
	Tables []string `json:"tables"`
}

// DescribeTableOutput is the result of describe_table.
type DescribeTableOutput struct {
	Columns []db.ColumnInfo `json:"columns"`
}

// RunQueryOutput is the result of run_query.
type RunQueryOutput struct {
	Rows []map[string]any `json:"rows"`
}

// InsertTestRowOutput is the result of insert_test_row.
type InsertTestRowOutput struct {
	InsertedID any `json:"inserted_id,omitempty"`
}

// UpdateTestRowOutput is the result of update_test_row.
type UpdateTestRowOutput struct {
	RowsAffected int64 `json:"rows_affected"`
}
