// Package server builds the MCP server and registers tools.
package server

import (
	"context"
	"fmt"

	"github.com/SedlarDavid/localdb-mcp/internal/config"
	"github.com/SedlarDavid/localdb-mcp/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ServerName    = "localdb-mcp"
	ServerVersion = "1.0.0"
)

// New returns an MCP server with all tools registered. cfg may be nil (only
// ping works without config); pass loaded config for list_connections and DB tools.
func New(cfg *config.Config) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    ServerName,
		Version: ServerVersion,
	}, nil)

	var mgr *db.Manager
	if cfg != nil {
		mgr = db.NewManager(cfg)
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "ping",
		Description: "Simple health check. Returns pong.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, PingOutput, error) {
		return nil, PingOutput{Message: "pong"}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_connections",
		Description: "List configured database connection IDs and their types (postgres, sqlserver). No credentials in response.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, ListConnectionsOutput, error) {
		out := ListConnectionsOutput{Connections: nil}
		if cfg != nil {
			out.Connections = cfg.ConnectionInfos()
		}
		return nil, out, nil
	})

	if mgr != nil {
		mcp.AddTool(s, &mcp.Tool{
			Name:        "list_tables",
			Description: "List table names in a given connection and optional schema.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, in ListTablesInput) (*mcp.CallToolResult, ListTablesOutput, error) {
			if in.ConnectionID == "" {
				return nil, ListTablesOutput{}, fmt.Errorf("connection_id is required")
			}
			driver, err := mgr.Driver(ctx, in.ConnectionID)
			if err != nil {
				return nil, ListTablesOutput{}, err
			}
			tables, err := driver.ListTables(ctx, in.Schema)
			if err != nil {
				return nil, ListTablesOutput{}, err
			}
			return nil, ListTablesOutput{Tables: tables}, nil
		})

		mcp.AddTool(s, &mcp.Tool{
			Name:        "describe_table",
			Description: "Describe columns of a table (name, type, nullable, primary key).",
		}, func(ctx context.Context, req *mcp.CallToolRequest, in DescribeTableInput) (*mcp.CallToolResult, DescribeTableOutput, error) {
			if in.ConnectionID == "" {
				return nil, DescribeTableOutput{}, fmt.Errorf("connection_id is required")
			}
			if in.Table == "" {
				return nil, DescribeTableOutput{}, fmt.Errorf("table is required")
			}
			driver, err := mgr.Driver(ctx, in.ConnectionID)
			if err != nil {
				return nil, DescribeTableOutput{}, err
			}
			cols, err := driver.DescribeTable(ctx, in.Schema, in.Table)
			if err != nil {
				return nil, DescribeTableOutput{}, err
			}
			return nil, DescribeTableOutput{Columns: cols}, nil
		})

		mcp.AddTool(s, &mcp.Tool{
			Name:        "run_query",
			Description: "Run a read-only SQL query (SELECT only). Rejects INSERT/UPDATE/DELETE/DDL. Params are positional.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, in RunQueryInput) (*mcp.CallToolResult, RunQueryOutput, error) {
			if in.ConnectionID == "" {
				return nil, RunQueryOutput{}, fmt.Errorf("connection_id is required")
			}
			if in.SQL == "" {
				return nil, RunQueryOutput{}, fmt.Errorf("sql is required")
			}
			if err := ValidateReadOnlySQL(in.SQL); err != nil {
				return nil, RunQueryOutput{}, err
			}
			driver, err := mgr.Driver(ctx, in.ConnectionID)
			if err != nil {
				return nil, RunQueryOutput{}, err
			}
			rows, err := driver.RunReadOnlyQuery(ctx, in.SQL, in.Params)
			if err != nil {
				return nil, RunQueryOutput{}, err
			}
			return nil, RunQueryOutput{Rows: rows}, nil
		})

		mcp.AddTool(s, &mcp.Tool{
			Name:        "insert_test_row",
			Description: "Insert a single test row. Optionally return generated ID (e.g. serial/identity).",
		}, func(ctx context.Context, req *mcp.CallToolRequest, in InsertTestRowInput) (*mcp.CallToolResult, InsertTestRowOutput, error) {
			if in.ConnectionID == "" {
				return nil, InsertTestRowOutput{}, fmt.Errorf("connection_id is required")
			}
			if in.Table == "" {
				return nil, InsertTestRowOutput{}, fmt.Errorf("table is required")
			}
			if len(in.Row) == 0 {
				return nil, InsertTestRowOutput{}, fmt.Errorf("row is required (object with column names and values)")
			}
			driver, err := mgr.Driver(ctx, in.ConnectionID)
			if err != nil {
				return nil, InsertTestRowOutput{}, err
			}
			id, err := driver.InsertRow(ctx, in.Schema, in.Table, in.Row)
			if err != nil {
				return nil, InsertTestRowOutput{}, err
			}
			out := InsertTestRowOutput{}
			if in.ReturnID && id != nil {
				out.InsertedID = id
			}
			return nil, out, nil
		})
	}

	return s
}

// PingOutput is the structured result of the ping tool.
type PingOutput struct {
	Message string `json:"message"`
}

// ListConnectionsOutput is the result of list_connections.
type ListConnectionsOutput struct {
	Connections []config.ConnectionInfo `json:"connections"`
}

// ListTablesInput is the input for list_tables.
type ListTablesInput struct {
	ConnectionID string `json:"connection_id"`
	Schema       string `json:"schema,omitempty"`
}

// ListTablesOutput is the result of list_tables.
type ListTablesOutput struct {
	Tables []string `json:"tables"`
}

// DescribeTableInput is the input for describe_table.
type DescribeTableInput struct {
	ConnectionID string `json:"connection_id"`
	Schema       string `json:"schema,omitempty"`
	Table        string `json:"table"`
}

// DescribeTableOutput is the result of describe_table.
type DescribeTableOutput struct {
	Columns []db.ColumnInfo `json:"columns"`
}

// RunQueryInput is the input for run_query.
type RunQueryInput struct {
	ConnectionID string `json:"connection_id"`
	SQL          string `json:"sql"`
	Params       []any  `json:"params,omitempty"`
}

// RunQueryOutput is the result of run_query.
type RunQueryOutput struct {
	Rows []map[string]any `json:"rows"`
}

// InsertTestRowInput is the input for insert_test_row.
type InsertTestRowInput struct {
	ConnectionID string            `json:"connection_id"`
	Schema       string            `json:"schema,omitempty"`
	Table        string            `json:"table"`
	Row          map[string]any `json:"row"`
	ReturnID     bool              `json:"return_id,omitempty"`
}

// InsertTestRowOutput is the result of insert_test_row.
type InsertTestRowOutput struct {
	InsertedID any `json:"inserted_id,omitempty"`
}
