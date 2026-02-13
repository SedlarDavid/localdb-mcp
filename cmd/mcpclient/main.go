// Package main runs a one-off MCP client: spawns the localdb-mcp server, calls
// one tool with optional JSON arguments, and prints the result. Run from repo root:
//
//	go run ./cmd/mcpclient <tool_name>              # no args, e.g. ping
//	go run ./cmd/mcpclient <tool_name> '<json>'    # with arguments
//
// Examples:
//
//	go run ./cmd/mcpclient ping
//	go run ./cmd/mcpclient list_connections
//	go run ./cmd/mcpclient list_tables '{"connection_id":"postgres"}'
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <tool_name> [json_arguments]\n", os.Args[0])
		os.Exit(1)
	}
	toolName := os.Args[1]
	var args map[string]interface{}
	if len(os.Args) >= 3 && os.Args[2] != "" {
		if err := json.Unmarshal([]byte(os.Args[2]), &args); err != nil {
			fmt.Fprintf(os.Stderr, "invalid json arguments: %v\n", err)
			os.Exit(1)
		}
	} else {
		args = make(map[string]interface{})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "find repo root: %v\n", err)
		os.Exit(1)
	}

	if err := os.Chdir(repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "chdir: %v\n", err)
		os.Exit(1)
	}

	env := os.Environ()

	c, err := client.NewStdioMCPClient("go", env, "run", "./cmd/server")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create client: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "mcpclient",
		Version: "1.0.0",
	}
	initReq.Params.Capabilities = mcp.ClientCapabilities{}

	if _, err := c.Initialize(ctx, initReq); err != nil {
		fmt.Fprintf(os.Stderr, "initialize: %v\n", err)
		os.Exit(1)
	}

	res, err := c.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "call tool: %v\n", err)
		os.Exit(1)
	}

	if res.IsError {
		msg := "tool failed"
		for _, content := range res.Content {
			if tc, ok := mcp.AsTextContent(content); ok {
				msg = tc.Text
				break
			}
		}
		fmt.Fprintf(os.Stderr, "tool error: %s\n", msg)
		os.Exit(1)
	}

	text := ""
	for _, content := range res.Content {
		if tc, ok := mcp.AsTextContent(content); ok {
			text += tc.Text
		}
	}
	fmt.Println(text)
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}
