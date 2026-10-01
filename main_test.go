package main

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestServer exercises the tools end to end against the live Chainguard
// endpoints. Skip with -short when offline.
func TestServer(t *testing.T) {
	if testing.Short() {
		t.Skip("hits the network")
	}
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := newServer(NewCatalog()).Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	call := func(name string, args any, out any) *mcp.CallToolResult {
		t.Helper()
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if out != nil && !res.IsError {
			b, _ := json.Marshal(res.StructuredContent)
			if err := json.Unmarshal(b, out); err != nil {
				t.Fatal(err)
			}
		}
		return res
	}

	var list ListImagesOutput
	call("list_images", map[string]any{"query": "python", "limit": 5}, &list)
	t.Logf("python images: total=%d page=%v", list.Total, list.Images)
	if list.Total == 0 || len(list.Images) == 0 || len(list.Images) > 5 {
		t.Fatalf("unexpected list result: %+v", list)
	}

	var all ListImagesOutput
	call("list_images", map[string]any{"limit": 1}, &all)
	t.Logf("total images: %d", all.Total)

	var tags GetImageTagsOutput
	call("get_image_tags", map[string]any{"image": "static"}, &tags)
	t.Logf("static tags: %v", tags.Tags)
	if !slices.Contains(tags.Tags, "latest") {
		t.Fatalf("expected latest tag, got %v", tags.Tags)
	}

	if res := call("get_image_tags", map[string]any{"image": "loki-fips"}, nil); !res.IsError {
		t.Fatal("expected error for non-public image")
	} else {
		t.Logf("loki-fips: %s", res.Content[0].(*mcp.TextContent).Text)
	}
}
