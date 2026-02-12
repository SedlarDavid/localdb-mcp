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
	ServerVersion = "0.1.0"
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
	}

	// TODO: run_query, insert_test_row (Phase 3)

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
