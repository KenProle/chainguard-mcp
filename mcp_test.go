package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpClient connects an in-memory MCP client to a server backed by svc.
func mcpClient(t *testing.T, svc *Service) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := newServer(svc).Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

// callTool calls a tool and decodes its structured output into out. It
// returns the error text if the tool reported an error.
func callTool(t *testing.T, s *mcp.ClientSession, name string, args map[string]any, out any) string {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		return res.Content[0].(*mcp.TextContent).Text
	}
	if out != nil {
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("%s: decoding output: %v", name, err)
		}
	}
	return ""
}

// mustCall is callTool for calls that are expected to succeed.
func mustCall(t *testing.T, s *mcp.ClientSession, name string, args map[string]any, out any) {
	t.Helper()
	if msg := callTool(t, s, name, args, out); msg != "" {
		t.Fatalf("%s returned error: %s", name, msg)
	}
}
