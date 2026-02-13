// Package main runs the localdb-mcp server: an MCP server that exposes
// read-only and controlled-write database tools to agents (e.g. Cursor)
// without exposing credentials.
package main

import (
	"log"
	"os"

	"github.com/SedlarDavid/localdb-mcp/internal/config"
	internal_server "github.com/SedlarDavid/localdb-mcp/internal/server"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Redirect logs to file for debugging if MCP_DEBUG is set
	if os.Getenv("MCP_DEBUG") != "" {
		f, err := os.OpenFile("/tmp/localdb-mcp.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err == nil {
			log.SetOutput(f)
			defer f.Close()
		}
	}
	
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Create MCP server
	s := server.NewMCPServer(
		internal_server.ServerName,
		internal_server.ServerVersion,
	)

	// Register tools
	internal_server.Register(s, cfg)

	if err := server.ServeStdio(s); err != nil {
		log.Printf("server error: %v", err)
	}
}
