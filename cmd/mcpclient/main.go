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
	"os/exec"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <tool_name> [json_arguments]\n", os.Args[0])
		os.Exit(1)
	}
	toolName := os.Args[1]
	var args any
	if len(os.Args) >= 3 && os.Args[2] != "" {
		if err := json.Unmarshal([]byte(os.Args[2]), &args); err != nil {
			fmt.Fprintf(os.Stderr, "invalid json arguments: %v\n", err)
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "find repo root: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/server")
	cmd.Dir = repoRoot
	cmd.Env = os.Environ() // pass through so server sees MCP_DB_* etc.
	cmd.Stderr = os.Stderr

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "stdin pipe: %v\n", err)
		os.Exit(1)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "stdout pipe: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "start server: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		stdinPipe.Close()
		_ = cmd.Wait()
	}()

	transport := &mcp.IOTransport{
		Reader: stdoutPipe,
		Writer: stdinPipe,
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "mcpclient", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "call tool: %v\n", err)
		os.Exit(1)
	}
	if res.IsError {
		fmt.Fprintf(os.Stderr, "tool error: %v\n", res.GetError())
		os.Exit(1)
	}
	text := ""
	if len(res.Content) > 0 {
		if tc, ok := res.Content[0].(*mcp.TextContent); ok {
			text = tc.Text
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
