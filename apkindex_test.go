package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const apkIndexFixture = "testdata/apkindex-2026-10-19.tar.gz"

// packageSample is the apt packages the mapping tests and the fixture cover:
// commonly installed ones that exist in Wolfi under the same name, ones dfc
// maps, ones only the rules find, and ones nothing finds.
var packageSample = strings.Fields(`
	curl git wget ca-certificates gcc make libxml2-dev libjpeg-dev openssh-client vim procps
	net-tools netcat-openbsd gnupg unzip tzdata npm yarn cmake jq sudo ffmpeg libpng-dev rsync
	libffi-dev bash openssl
	build-essential libssl-dev libsqlite3-dev ssh zlib1g-dev pkg-config locales libcurl4-openssl-dev
	python3-pip libc6-dev g++ fonts-liberation software-properties-common
	libpq-dev python3 nodejs postgresql-client libgomp1 iputils-ping default-libmysqlclient-dev
	python3-dev libgl1 mysql-client libmariadb-dev nginx imagemagick
	dnsutils python3-venv default-jre libglib2.0-0 redis-tools python3-numpy
	golang-go`)

func loadFixtureIndex(t *testing.T) *IndexData {
	t.Helper()
	f, err := os.Open(apkIndexFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	idx, err := parseAPKIndex(f)
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

// TestUpdateAPKIndexFixture regenerates testdata/apkindex-2026-10-19.tar.gz,
// a trimmed copy of Wolfi's real x86_64 index with every build of the packages
// the tests touch. APKINDEX_SOURCE names a downloaded APKINDEX.tar.gz; without
// it the live index is downloaded.
//
//	UPDATE_FIXTURES=1 go test -run TestUpdateAPKIndexFixture ./...
func TestUpdateAPKIndexFixture(t *testing.T) {
	if os.Getenv("UPDATE_FIXTURES") == "" {
		t.Skip("set UPDATE_FIXTURES=1 to regenerate")
	}
	var full []byte
	if src := os.Getenv("APKINDEX_SOURCE"); src != "" {
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		full = b
	} else {
		resp, err := http.Get(defaultPackageIndexURL)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if full, err = io.ReadAll(resp.Body); err != nil {
			t.Fatal(err)
		}
	}
	whole, err := parseAPKIndex(bytes.NewReader(full))
	if err != nil {
		t.Fatal(err)
	}
	dfc := mustLoadDFCMappings()

	keep := map[string]bool{}
	streams := regexp.MustCompile(`^((nodejs|postgresql|python|mysql|mariadb)-\d|python3-as-)`)
	for name := range whole.Pkgs {
		if streams.MatchString(name) {
			keep[name] = true
		}
	}
	keep["gcc-go"] = true // provides cmd:go, which golang-go must not map to
	want := map[string]PackageMapping{}
	for _, n := range packageSample {
		m := mapPackage(whole, dfc.Debian, n)
		want[n] = m
		for _, p := range append(append([]string{}, m.Packages...), m.Alternatives...) {
			keep[p] = true
		}
		keep[n] = true
		for _, tgt := range dfc.Debian[n] {
			keep[tgt] = true
			keep[whole.Pkgs[tgt].Origin] = true
		}
	}

	// Copy every build of the kept packages.
	zr, err := gzip.NewReader(bytes.NewReader(full))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(zr)
	var text io.Reader
	for {
		hdr, err := tr.Next()
		if err != nil {
			t.Fatal("no APKINDEX member")
		}
		if hdr.Name == "APKINDEX" {
			text = tr
			break
		}
	}
	var out bytes.Buffer
	var record []string
	flush := func() {
		if len(record) == 0 {
			return
		}
		for _, l := range record {
			if strings.HasPrefix(l, "P:") && keep[l[2:]] {
				out.WriteString(strings.Join(record, "\n") + "\n\n")
			}
		}
		record = nil
	}
	sc := bufio.NewScanner(text)
	sc.Buffer(make([]byte, 1<<20), 8<<20)
	for sc.Scan() {
		if sc.Text() == "" {
			flush()
			continue
		}
		record = append(record, sc.Text())
	}
	flush()

	trimmed, err := parseAPKIndexText(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range packageSample {
		if got := mapPackage(trimmed, dfc.Debian, n); !reflect.DeepEqual(got, want[n]) {
			t.Errorf("%s: trimmed index gives %+v, full index gives %+v", n, got, want[n])
		}
	}
	if t.Failed() {
		return
	}

	var gz bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&gz, gzip.BestCompression)
	tw := tar.NewWriter(zw)
	tw.WriteHeader(&tar.Header{Name: "APKINDEX", Mode: 0o644, Size: int64(out.Len())})
	tw.Write(out.Bytes())
	tw.Close()
	zw.Close()
	if err := os.WriteFile(apkIndexFixture, gz.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s: %d packages, %d bytes", apkIndexFixture, len(trimmed.Pkgs), gz.Len())
}

// servePackageIndex serves the fixture and counts downloads.
func servePackageIndex(t *testing.T) (*httptest.Server, *atomic.Int64, *atomic.Bool) {
	t.Helper()
	data, err := os.ReadFile(apkIndexFixture)
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int64
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if fail.Load() {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv, &requests, &fail
}

func TestAPKIndexParsesFixture(t *testing.T) {
	idx := loadFixtureIndex(t)
	if got := idx.Pkgs["libssl3"].Origin; got != "openssl" {
		t.Errorf("libssl3 origin = %q, want openssl", got)
	}
	var streams []string
	for _, p := range idx.Providers("pc:libpq") {
		if strings.HasPrefix(p, "postgresql-") && strings.HasSuffix(p, "-dev") {
			streams = append(streams, p)
		}
	}
	if len(streams) < 3 {
		t.Errorf("pc:libpq is provided by %v, want several postgresql-*-dev streams", streams)
	}
	if got := idx.ProvidersFold("so:libgl.so.1"); len(got) == 0 {
		t.Errorf("a lower-cased so: token finds nothing")
	}
}

func TestPM10_1_IndexDownloadedOnce(t *testing.T) {
	srv, requests, _ := servePackageIndex(t)
	p := NewPackageIndex(srv.Client())
	p.URL = srv.URL
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	p.Now = func() time.Time { return now }
	if _, _, err := p.Get(context.Background()); err != nil {
		t.Fatal(err)
	}
	now = now.Add(10 * time.Minute)
	if _, _, err := p.Get(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 1 {
		t.Errorf("%d downloads, want 1", got)
	}
}

func TestPM10_2_FailedRefreshServesOldCopy(t *testing.T) {
	srv, _, fail := servePackageIndex(t)
	p := NewPackageIndex(srv.Client())
	p.URL = srv.URL
	start := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	now := start
	p.Now = func() time.Time { return now }
	if _, _, err := p.Get(context.Background()); err != nil {
		t.Fatal(err)
	}
	now = start.Add(7 * time.Hour)
	fail.Store(true)
	idx, fetched, err := p.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if idx == nil || !fetched.Equal(start) {
		t.Errorf("fetched = %v, want the old copy's time %v", fetched, start)
	}
}

func TestPM11_1_NoIndexIsSourceError(t *testing.T) {
	srv, _, fail := servePackageIndex(t)
	fail.Store(true)
	p := NewPackageIndex(srv.Client())
	p.URL = srv.URL
	_, _, err := p.Get(context.Background())
	var srcErr *SourceError
	if !errors.As(err, &srcErr) || !strings.Contains(srcErr.Source, "Wolfi package index") {
		t.Fatalf("got %v, want a Wolfi package index SourceError", err)
	}
}
