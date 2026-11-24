package main

import (
	"context"
	"errors"
	"fmt"
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

// errAfterReader returns data, then a non-EOF error, like a connection reset.
type errAfterReader struct {
	data string
	err  error
}

func (r *errAfterReader) Read(p []byte) (int, error) {
	if r.data == "" {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func TestDecodeJSONLimitedKeepsCompleteValue(t *testing.T) {
	var v struct{ A string }
	reset := errors.New("connection reset")
	if err := decodeJSONLimited(&errAfterReader{`{"A":"b"}`, reset}, 1<<10, &v); err != nil || v.A != "b" {
		t.Fatalf("got %v, %+v", err, v)
	}
	// A reset in the middle of the value is still an error.
	if err := decodeJSONLimited(&errAfterReader{`{"A":"`, reset}, 1<<10, &v); err == nil {
		t.Fatal("a cut-off value decoded")
	}
}

// TestOSVPagingIsBounded: a server that always names a next page, or keeps
// returning advisories, ends in an error instead of growing without limit.
func TestOSVPagingIsBounded(t *testing.T) {
	pkgs := []OSVPackage{{Ecosystem: "Debian:12", Name: "openssl", Version: "1"}}
	run := func(t *testing.T, vulnsPerPage int) (int, error) {
		pages := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pages++
			var items []string
			for i := range vulnsPerPage {
				items = append(items, fmt.Sprintf(`{"id":"DSA-%d-%d"}`, pages, i))
			}
			fmt.Fprintf(w, `{"vulns":[%s],"next_page_token":"more"}`, strings.Join(items, ","))
		}))
		defer srv.Close()
		o := NewOSV(srv.Client())
		o.URL = srv.URL
		_, err := o.QueryAll(context.Background(), pkgs, nil)
		return pages, err
	}
	t.Run("pages", func(t *testing.T) {
		pages, err := run(t, 1)
		var srcErr *SourceError
		if !errors.As(err, &srcErr) || !errors.Is(err, errTooLarge) {
			t.Fatalf("got %v", err)
		}
		if pages != maxOSVPages {
			t.Fatalf("made %d requests, want %d", pages, maxOSVPages)
		}
	})
	t.Run("advisories", func(t *testing.T) {
		_, err := run(t, maxOSVVulns/2+1)
		var srcErr *SourceError
		if !errors.As(err, &srcErr) || !errors.Is(err, errTooLarge) {
			t.Fatalf("got %v", err)
		}
	})
}
