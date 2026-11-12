package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// oversized serves prefix followed by padding until total bytes (at least one
// more than any cap), never ending the value, so only the cap can stop it.
func oversized(prefix string, total int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, prefix)
		chunk := []byte(strings.Repeat("x", 64<<10))
		for n := int64(len(prefix)); n < total; n += int64(len(chunk)) {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}
}

func TestLimitedReader(t *testing.T) {
	// Exactly at the cap is fine; one byte more is an error, not a short read.
	if b, err := io.ReadAll(limitedReader(strings.NewReader("abcd"), 4)); err != nil || string(b) != "abcd" {
		t.Fatalf("at cap: %q, %v", b, err)
	}
	if _, err := io.ReadAll(limitedReader(strings.NewReader("abcde"), 4)); !errors.Is(err, errTooLarge) {
		t.Fatalf("over cap: %v", err)
	}
	var v struct{ A string }
	if err := decodeJSONLimited(strings.NewReader(`{"A":"b"}`+strings.Repeat(" ", 100)), 20, &v); !errors.Is(err, errTooLarge) {
		t.Fatalf("trailing padding over cap: %v", err)
	}
}

// TestResponseSizeCaps checks that every source refuses a body over its cap
// with an error rather than decoding a truncated one.
func TestResponseSizeCaps(t *testing.T) {
	ctx := context.Background()
	client := http.DefaultClient

	t.Run("sitemap", func(t *testing.T) {
		srv := httptest.NewServer(oversized(`<urlset><!-- `, maxSitemapBytes+1))
		defer srv.Close()
		c := NewCatalog(client)
		c.SitemapURL = srv.URL
		if _, err := c.Images(ctx); !errors.Is(err, errTooLarge) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("secdb", func(t *testing.T) {
		srv := httptest.NewServer(oversized(`{"packages":[{"pkg":{"name":"`, maxSecDBBytes+1))
		defer srv.Close()
		s := NewSecDB(client)
		s.URL = srv.URL
		if _, err := s.fetch(ctx); !errors.Is(err, errTooLarge) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("endoflife", func(t *testing.T) {
		srv := httptest.NewServer(oversized(`[{"cycle":"`, maxEndOfLifeJSON+1))
		defer srv.Close()
		e := NewEndOfLife(client)
		e.URL = srv.URL
		if _, err := e.cycles(ctx, "nodejs"); !errors.Is(err, errTooLarge) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("osv", func(t *testing.T) {
		srv := httptest.NewServer(oversized(`{"vulns":[{"id":"`, maxOSVPageBytes+1))
		defer srv.Close()
		o := NewOSV(client)
		o.URL = srv.URL
		_, err := o.QueryAll(ctx, []OSVPackage{{Ecosystem: "Debian:12", Name: "openssl", Version: "1"}}, nil)
		var srcErr *SourceError
		if !errors.As(err, &srcErr) || !errors.Is(err, errTooLarge) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("registry token", func(t *testing.T) {
		srv := httptest.NewServer(oversized(`{"token":"`, maxTokenBytes+1))
		defer srv.Close()
		r := NewRegistry(client)
		r.BaseURL = srv.URL
		if _, err := r.Tags(ctx, "python"); !errors.Is(err, errTooLarge) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("registry json", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"token":"t","expires_in":300}`)
		})
		mux.HandleFunc("/", oversized(`{"tags":["`, maxRegistryJSON+1))
		srv := httptest.NewServer(mux)
		defer srv.Close()
		r := NewRegistry(client)
		r.BaseURL = srv.URL
		if _, err := r.Tags(ctx, "python"); !errors.Is(err, errTooLarge) {
			t.Fatalf("got %v", err)
		}
	})
}
