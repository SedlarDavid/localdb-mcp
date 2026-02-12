// Package main runs the localdb-mcp server: an MCP server that exposes
// read-only and controlled-write database tools to agents (e.g. Cursor)
// without exposing credentials.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/SedlarDavid/localdb-mcp/internal/config"
	"github.com/SedlarDavid/localdb-mcp/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	srv := server.New(cfg)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil && context.Cause(ctx) != context.Canceled {
		log.Fatalf("server: %v", err)
	}
}
