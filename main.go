// Command chainguard-mcp is an MCP server for discovering Chainguard
// container images.
//
// By default it speaks MCP over stdio. Pass -http 127.0.0.1:8080 to serve
// the Streamable HTTP transport at /mcp instead, along with a web UI at /
// and its JSON API under /api/.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is set at build time by GoReleaser (-ldflags "-X main.version=…").
var version = "dev"

func main() {
	httpAddr := flag.String("http", "", "if set, serve Streamable HTTP on this address (e.g. 127.0.0.1:8080) instead of stdio")
	showVersion := flag.Bool("version", false, "print the version and exit")
	sbomDir := flag.String("sbom-dir", "", "folder where save_sbom writes files (default: the current folder in stdio mode; disabled in HTTP mode unless set)")
	recordFile := flag.String("record-history", "", "take a live catalog snapshot, append it to this history file and exit")
	backfillFile := flag.String("backfill-history", "", "build this empty history file from the Internet Archive's sitemap captures and exit")
	refreshFile := flag.String("refresh-mappings", "", "download dfc's latest package mapping file to this path (with dfc-source.json and LICENSE-dfc beside it), print what changed and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}
	if *refreshFile != "" {
		r := NewDFCRefresher(&http.Client{Timeout: 60 * time.Second})
		r.Now = func() string { return time.Now().UTC().Format("2006-01-02") }
		report, err := r.Refresh(context.Background(), *refreshFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Print(report)
		return
	}
	if *recordFile != "" || *backfillFile != "" {
		if err := runHistory(NewService(), NewArchive(&http.Client{Timeout: 60 * time.Second}), *recordFile, *backfillFile); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
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
		mux.Handle("/", newWebHandler(svc, embeddedUI()))
		log.Printf("chainguard-mcp %s: web UI at http://%s/, MCP at http://%s/mcp", version, *httpAddr, *httpAddr)
		// No write timeout: /mcp streams responses and comparisons can take a
		// while. The header limits stop slow or oversized requests.
		srv := &http.Server{
			Addr:              *httpAddr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       2 * time.Minute,
			MaxHeaderBytes:    64 << 10,
		}
		log.Fatal(srv.ListenAndServe())
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
