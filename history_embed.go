package main

import "embed"

// historyData holds the catalog history embedded at build time. The folder
// always contains .gitkeep, so the Go build works without the history file;
// release builds and the Docker image download it into historydata/ first
// (see the Dockerfile and .goreleaser.yaml).
//
//go:embed all:historydata
var historyData embed.FS

// embeddedHistory returns the history file embedded at build time, or nil
// when the build has none.
func embeddedHistory() []byte {
	b, err := historyData.ReadFile("historydata/catalog-history.jsonl")
	if err != nil {
		return nil
	}
	return b
}
