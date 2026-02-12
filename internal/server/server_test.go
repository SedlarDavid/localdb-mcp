package server

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPingTool(t *testing.T) {
	ctx := context.Background()
	serverTrans, clientTrans := mcp.NewInMemoryTransports()

	srv := New(nil)
	go func() {
		_ = srv.Run(ctx, serverTrans)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, clientTrans, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	// List tools and ensure ping is present
	var found bool
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("Tools iterator: %v", err)
		}
		if tool.Name == "ping" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ping tool in list")
	}

	// Call ping
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "ping",
		Arguments: nil,
	})
	if err != nil {
		t.Fatalf("CallTool(ping): %v", err)
	}
	if res.IsError {
		t.Errorf("ping returned error: %v", res.GetError())
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
	if tc, ok := res.Content[0].(*mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}
