// Command chainguard-mcp is an MCP server for discovering Chainguard
// container images.
//
// By default it speaks MCP over stdio. Pass -http :8080 to serve the
// Streamable HTTP transport at /mcp instead.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListImagesInput struct {
	Query  string `json:"query,omitempty" jsonschema:"case-insensitive substring to filter image names, e.g. 'python' or 'fips'"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of images to return (default 100, max 1000)"`
	Offset int    `json:"offset,omitempty" jsonschema:"number of matching images to skip, for pagination"`
}

type ListImagesOutput struct {
	Total  int      `json:"total" jsonschema:"number of images matching the query"`
	Count  int      `json:"count" jsonschema:"number of images in this page"`
	Offset int      `json:"offset"`
	Images []string `json:"images" jsonschema:"image names; pull as cgr.dev/chainguard/<name>"`
}

type GetImageTagsInput struct {
	Image string `json:"image" jsonschema:"image name as returned by list_images, e.g. 'python'"`
}

type GetImageTagsOutput struct {
	Image     string   `json:"image"`
	Reference string   `json:"reference" jsonschema:"full registry reference for the image"`
	Tags      []string `json:"tags"`
}

func newServer(cat *Catalog) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "chainguard-images", Version: "0.1.0"}, nil)
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_images",
		Description: "List available Chainguard container images from the Chainguard Images directory, optionally filtered by a name substring.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListImagesInput) (*mcp.CallToolResult, ListImagesOutput, error) {
		all, err := cat.Images(ctx)
		if err != nil {
			return nil, ListImagesOutput{}, err
		}
		q := strings.ToLower(strings.TrimSpace(in.Query))
		var matches []string
		for _, name := range all {
			if q == "" || strings.Contains(name, q) {
				matches = append(matches, name)
			}
		}

		limit := in.Limit
		if limit <= 0 {
			limit = 100
		}
		limit = min(limit, 1000)
		offset := min(max(in.Offset, 0), len(matches))
		page := matches[offset:min(offset+limit, len(matches))]
		if page == nil {
			page = []string{}
		}
		return nil, ListImagesOutput{Total: len(matches), Count: len(page), Offset: offset, Images: page}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_image_tags",
		Description: "List the tags of a Chainguard image on cgr.dev. Only works for publicly available (free tier) images.",
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in GetImageTagsInput) (*mcp.CallToolResult, GetImageTagsOutput, error) {
		image := strings.TrimSpace(in.Image)
		if image == "" || strings.ContainsAny(image, "/:@ ") {
			return nil, GetImageTagsOutput{}, fmt.Errorf("invalid image name %q", in.Image)
		}
		tags, err := cat.Tags(ctx, image)
		if errors.Is(err, ErrNotPublic) {
			return nil, GetImageTagsOutput{}, fmt.Errorf("%s: %w", image, err)
		}
		if err != nil {
			return nil, GetImageTagsOutput{}, err
		}
		if tags == nil {
			tags = []string{}
		}
		return nil, GetImageTagsOutput{Image: image, Reference: "cgr.dev/" + defaultRepoPrefix + "/" + image, Tags: tags}, nil
	})

	return server
}

func main() {
	httpAddr := flag.String("http", "", "if set, serve Streamable HTTP on this address (e.g. :8080) instead of stdio")
	flag.Parse()

	server := newServer(NewCatalog())

	if *httpAddr != "" {
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
		mux := http.NewServeMux()
		mux.Handle("/mcp", handler)
		log.Printf("chainguard-mcp listening on %s/mcp", *httpAddr)
		log.Fatal(http.ListenAndServe(*httpAddr, mux))
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
