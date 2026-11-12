package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Size caps for responses read from other servers, so a broken or compromised
// upstream can't make the process buffer an unbounded body. Each is a
// generous multiple of the largest response measured on 2026-11-12:
//
//	registry token        5.5 KB (Docker Hub)       -> 64 KB
//	registry JSON         155 KB (tag list, Docker  -> 8 MB
//	                      Hub node; manifests are a few KB)
//	OSV.dev query page    650 KB (Debian openssl)   -> 8 MB
//	Wolfi security DB     7.7 MB                    -> 64 MB
//	images sitemap        1.6 MB                    -> 16 MB
//	endoflife.date        5 KB per product          -> 1 MB
//
// Already capped elsewhere: Registry.Blob (caller's maxBytes), the advisory
// feed (4 MB), the package index (maxIndexBytes), the history file and the
// dfc refresh.
const (
	maxTokenBytes    = 64 << 10
	maxRegistryJSON  = 8 << 20
	maxOSVPageBytes  = 8 << 20
	maxSecDBBytes    = 64 << 20
	maxSitemapBytes  = 16 << 20
	maxEndOfLifeJSON = 1 << 20
)

// errTooLarge is wrapped by every cap failure, so a response over its limit is
// an error and never silently truncated data.
var errTooLarge = errors.New("response too large")

// limitedReader returns r capped at max bytes; reading past the cap fails with
// errTooLarge instead of reporting a clean end of input.
func limitedReader(r io.Reader, max int64) io.Reader {
	return &capReader{r: r, left: max, max: max}
}

type capReader struct {
	r         io.Reader
	left, max int64
}

func (c *capReader) Read(p []byte) (int, error) {
	if c.left <= 0 {
		// At the cap: only a clean end of input is acceptable.
		var one [1]byte
		n, err := c.r.Read(one[:])
		if n > 0 {
			return 0, fmt.Errorf("%w: more than %d bytes", errTooLarge, c.max)
		}
		return 0, err
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	return n, err
}

// decodeJSONLimited decodes one JSON value from r, failing with errTooLarge if
// the body exceeds max bytes.
func decodeJSONLimited(r io.Reader, max int64, v any) error {
	cr := limitedReader(r, max)
	if err := json.NewDecoder(cr).Decode(v); err != nil {
		return err
	}
	// Drain the rest (trailing whitespace is fine) so an oversized body is
	// reported even when the value itself decoded within the cap.
	_, err := io.Copy(io.Discard, cr)
	return err
}
