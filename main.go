// Command chainguard-mcp is an MCP server for discovering Chainguard
// container images.
//
// By default it speaks MCP over stdio. Pass -http 127.0.0.1:8080 to serve
// the Streamable HTTP transport at /mcp instead.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is set at build time by GoReleaser (-ldflags "-X main.version=…").
var version = "dev"

func main() {
	httpAddr := flag.String("http", "", "if set, serve Streamable HTTP on this address (e.g. 127.0.0.1:8080) instead of stdio")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	server := newServer(NewService())

	if *httpAddr != "" {
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
		mux := http.NewServeMux()
		mux.Handle("/mcp", handler)
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, "ok") })
		log.Printf("chainguard-mcp %s listening on %s/mcp", version, *httpAddr)
		log.Fatal(http.ListenAndServe(*httpAddr, mux))
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
