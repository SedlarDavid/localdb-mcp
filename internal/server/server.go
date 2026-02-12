// Package server builds the MCP server and registers tools.
package server

import (
	"context"

	"github.com/SedlarDavid/localdb-mcp/internal/config"
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

	// TODO: list_tables, describe_table, run_query, insert_test_row (Phase 3)

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
