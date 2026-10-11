package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func newCompareServer(t *testing.T) (*httptest.Server, *Service, *fakeUpstream) {
	t.Helper()
	svc, f := newFakeUpstreamService(t)
	srv := httptest.NewServer(newWebHandler(svc, fakeUI))
	t.Cleanup(srv.Close)
	return srv, svc, f
}

// pollCompare polls path until the job finishes or fails, as the UI does.
func pollCompare(t *testing.T, srv *httptest.Server, path string) (int, []byte) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var s struct{ Status string }
		json.Unmarshal(body, &s)
		if resp.StatusCode != 200 || s.Status != "running" {
			return resp.StatusCode, body
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s: still running after 10 seconds", path)
	return 0, nil
}

func TestWA_CompareUpstreamRunsThenDone(t *testing.T) {
	srv, svc, f := newCompareServer(t)
	f.OSV.delay = 100 * time.Millisecond
	const path = "/api/compare-upstream?image=node:20"

	var first CompareJobStatus
	if resp := getJSON(t, srv, path, &first); resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if first.Status != "running" || first.UpstreamComparison != nil {
		t.Fatalf("first call = %+v, want running", first)
	}

	code, body := pollCompare(t, srv, path)
	if code != 200 {
		t.Fatalf("status %d: %s", code, body)
	}
	var done CompareJobStatus
	if err := json.Unmarshal(body, &done); err != nil {
		t.Fatal(err)
	}
	if done.Status != "done" || done.UpstreamComparison == nil || done.Total != 6 || done.Checked != 6 {
		t.Fatalf("done = %+v", done)
	}

	// The same comparison as Service.CompareUpstream.
	direct, err := svc.CompareUpstream(context.Background(), CompareUpstreamInput{Image: "node:20"})
	if err != nil {
		t.Fatal(err)
	}
	direct.CheckedAt, done.CheckedAt = "", ""
	a, _ := json.Marshal(direct)
	b, _ := json.Marshal(done.UpstreamComparison)
	if string(a) != string(b) {
		t.Errorf("job result differs from CompareUpstream:\n%s\n%s", b, a)
	}
	if done.Upstream.Vulnerabilities.FixAvailable != 1 {
		t.Errorf("upstream vulnerabilities = %+v", done.Upstream.Vulnerabilities)
	}

	// A poll of the finished job returns the result again without new work.
	before := f.OSV.requests.Load()
	var again CompareJobStatus
	getJSON(t, srv, path, &again)
	if again.Status != "done" || f.OSV.requests.Load() != before {
		t.Errorf("repeat poll = %s, extra OSV requests %d", again.Status, f.OSV.requests.Load()-before)
	}
}

func TestWA_CompareUpstreamConcurrentCallsShareOneJob(t *testing.T) {
	srv, _, f := newCompareServer(t)
	f.OSV.delay = 50 * time.Millisecond
	const path = "/api/compare-upstream?image=node:20&arch=amd64"

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if code, body := pollCompare(t, srv, path); code != 200 || !strings.Contains(string(body), `"done"`) {
				t.Errorf("%d %s", code, body)
			}
		})
	}
	wg.Wait()
	if n := f.OSV.requests.Load(); n != 6 {
		t.Errorf("OSV requests = %d, want 6 (one job)", n)
	}
	if n := f.Hub.blobRequests.Load(); n != 2 { // image config and SBOM
		t.Errorf("Docker Hub blob requests = %d, want 2 (one job)", n)
	}
}

func TestWA_CompareUpstreamErrors(t *testing.T) {
	check := func(t *testing.T, srv *httptest.Server, path string, status int, code, source string) {
		t.Helper()
		got, body := pollCompare(t, srv, path)
		var e apiError
		json.Unmarshal(body, &e)
		if got != status || e.Code != code || e.Source != source {
			t.Errorf("%s: %d %s, want %d %s %q", path, got, body, status, code, source)
		}
	}

	t.Run("invalid input fails at once", func(t *testing.T) {
		srv, _, _ := newCompareServer(t)
		check(t, srv, "/api/compare-upstream", 400, "invalid_input", "")
		check(t, srv, "/api/compare-upstream?image=No+Good", 400, "invalid_input", "")
		check(t, srv, "/api/compare-upstream?image=node:20&arch=../x", 400, "invalid_input", "")
	})
	t.Run("UC-2.3 other registry", func(t *testing.T) {
		srv, _, _ := newCompareServer(t)
		check(t, srv, "/api/compare-upstream?image=ghcr.io/example/app:1", 400, "unsupported_registry", "")
	})
	t.Run("UC-1.2 no alternative", func(t *testing.T) {
		srv, _, _ := newCompareServer(t)
		check(t, srv, "/api/compare-upstream?image=some-unknown-app:1", 404, "no_alternative", "")
	})
	t.Run("not on Docker Hub", func(t *testing.T) {
		srv, _, _ := newCompareServer(t)
		check(t, srv, "/api/compare-upstream?image=node:99", 404, "not_found", "")
	})
	t.Run("UC-11.1 rate limited", func(t *testing.T) {
		srv, _, f := newCompareServer(t)
		f.Hub.limited.Store(true)
		check(t, srv, "/api/compare-upstream?image=node:20", 429, "rate_limited", "")
	})
	t.Run("UC-11.2 OSV failure, then a retry succeeds", func(t *testing.T) {
		srv, _, f := newCompareServer(t)
		f.OSV.failName = map[string]bool{"glibc": true}
		const path = "/api/compare-upstream?image=node:20"
		check(t, srv, path, 502, "source_failed", "OSV.dev")
		// The failed job is forgotten once reported, so the next call retries.
		f.OSV.failName = nil
		if code, body := pollCompare(t, srv, path); code != 200 || !strings.Contains(string(body), `"done"`) {
			t.Errorf("retry: %d %s", code, body)
		}
	})
	t.Run("UC-11.3 advisory feed failure, then a retry succeeds", func(t *testing.T) {
		srv, _, f := newCompareServer(t)
		f.Feed.failID = map[string]bool{"CGA-0000-0000-0001": true}
		const path = "/api/compare-upstream?image=node:20"
		check(t, srv, path, 502, "source_failed", "Chainguard advisories")
		f.Feed.failID = nil
		if code, body := pollCompare(t, srv, path); code != 200 || !strings.Contains(string(body), `"done"`) {
			t.Errorf("retry: %d %s", code, body)
		}
	})
}
