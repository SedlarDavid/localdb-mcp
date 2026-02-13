package server

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestPingTool(t *testing.T) {
	ctx := context.Background()

	// Create server and register tools (nil config = only ping + list_connections)
	s := server.NewMCPServer(ServerName, ServerVersion)
	Register(s, nil)

	// Create in-process client
	c, err := client.NewInProcessClient(s)
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}
	defer c.Close()

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "test-client", Version: "1.0.0"}

	if _, err := c.Initialize(ctx, initReq); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// List tools and ensure ping is present
	toolsRes, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	var found bool
	for _, tool := range toolsRes.Tools {
		if tool.Name == "ping" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ping tool in list")
	}

	// Call ping
	res, err := c.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "ping",
			Arguments: map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("CallTool(ping): %v", err)
	}
	if res.IsError {
		t.Errorf("ping returned error")
	}
	text := textContent(res)
	if text != `{"message":"pong"}` {
		t.Errorf("ping result: got %q, want {\"message\":\"pong\"}", text)
	}
}

func textContent(res *mcp.CallToolResult) string {
	if res == nil || len(res.Content) == 0 {
		return ""
	}
	if tc, ok := mcp.AsTextContent(res.Content[0]); ok {
		return tc.Text
	}
	return ""
}
