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
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is set at build time by GoReleaser (-ldflags "-X main.version=…").
var version = "dev"

func main() {
	httpAddr := flag.String("http", "", "if set, serve Streamable HTTP on this address (e.g. 127.0.0.1:8080) instead of stdio")
	showVersion := flag.Bool("version", false, "print the version and exit")
	sbomDir := flag.String("sbom-dir", "", "folder where save_sbom writes files (default: the current folder in stdio mode; disabled in HTTP mode unless set)")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	svc := NewService()
	switch {
	case *sbomDir != "":
		svc.SBOMDir = *sbomDir
	case *httpAddr == "":
		// In stdio mode the server runs locally for one user, so saving to
		// the current folder is safe. Over HTTP, remote clients would be
		// writing to this machine, so saving stays off unless configured.
		if wd, err := os.Getwd(); err == nil {
			svc.SBOMDir = wd
		}
	}
	if svc.SBOMDir != "" {
		abs, err := filepath.Abs(svc.SBOMDir)
		if err != nil {
			log.Fatalf("invalid -sbom-dir: %v", err)
		}
		svc.SBOMDir = abs
	}
	server := newServer(svc)

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
