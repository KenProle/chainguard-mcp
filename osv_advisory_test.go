package main

import (
	"context"
	"testing"
	"time"
)

func TestOA_DecodeChainguardAndDebianRecords(t *testing.T) {
	cg := loadOSVFixture[OSVVuln](t, "osv-chainguard-glibc.json")
	if len(cg.Upstream) != 2 || cg.Upstream[0] != "CVE-2026-89092" {
		t.Errorf("upstream = %v", cg.Upstream)
	}
	comp := cg.Affected[1].EcosystemSpecific.Components[0]
	if comp.ComponentName != "glibc-2.44" || comp.ComponentType != "apk" || comp.LatestEventStatus != "pending_upstream_fix" {
		t.Errorf("component = %+v", comp)
	}
	if got := cg.fixedVersion(OSVPackage{"Wolfi", "glibc-2.44", "2.44-r8"}); got != "" {
		t.Errorf("chainguard fixed version = %q, want none", got)
	}

	deb := loadOSVFixture[map[string]OSVVuln](t, "osv-debian-openssl.json")
	if got := deb["fixed"].fixedVersion(openssl12); got == "" {
		t.Error("debian record should name a fixed version for Debian 12")
	}
	if got := deb["fixed"].fixedVersion(OSVPackage{"Debian:12", "other", "1"}); got != "" {
		t.Errorf("fixed version for another package = %q", got)
	}
}

// clock is a controllable time source for cache tests.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func TestOA12_1_RepeatWithinAnHourIsCached(t *testing.T) {
	o, fake := newFakeOSV(t)
	clk := &clock{t: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)}
	o.Now = clk.now
	pkgs := []OSVPackage{{"Wolfi", "glibc-2.44", "2.44-r8"}, {"Wolfi", "python-3.15", "3.15.0-r0"}}

	_, first, err := o.QueryAllAt(context.Background(), pkgs, nil)
	if err != nil || !first.Equal(clk.t) {
		t.Fatalf("first = %v, %v", first, err)
	}
	clk.t = clk.t.Add(59 * time.Minute)
	before := fake.requests.Load()
	_, second, err := o.QueryAllAt(context.Background(), pkgs, nil)
	if err != nil || fake.requests.Load() != before {
		t.Fatalf("repeat made %d requests, err %v", fake.requests.Load()-before, err)
	}
	if !second.Equal(first) {
		t.Errorf("checked time = %v, want the first fetch %v", second, first)
	}
}

func TestOA12_2_AfterAnHourQueriesAgain(t *testing.T) {
	o, fake := newFakeOSV(t)
	clk := &clock{t: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)}
	o.Now = clk.now
	pkgs := []OSVPackage{{"Wolfi", "glibc-2.44", "2.44-r8"}}
	if _, _, err := o.QueryAllAt(context.Background(), pkgs, nil); err != nil {
		t.Fatal(err)
	}
	clk.t = clk.t.Add(61 * time.Minute)
	before := fake.requests.Load()
	_, at, err := o.QueryAllAt(context.Background(), pkgs, nil)
	if err != nil || fake.requests.Load() != before+1 || !at.Equal(clk.t) {
		t.Errorf("requests +%d, at %v, err %v; want one request at %v", fake.requests.Load()-before, at, err, clk.t)
	}
}

func TestOA12_UC10_DebianStaysCachedAt23Hours(t *testing.T) {
	o, fake := newFakeOSV(t)
	clk := &clock{t: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)}
	o.Now = clk.now
	if _, err := o.Query(context.Background(), openssl12); err != nil {
		t.Fatal(err)
	}
	clk.t = clk.t.Add(23 * time.Hour)
	before := fake.requests.Load()
	if _, err := o.Query(context.Background(), openssl12); err != nil || fake.requests.Load() != before {
		t.Errorf("Debian lookup at 23 hours made %d requests, err %v", fake.requests.Load()-before, err)
	}
	clk.t = clk.t.Add(2 * time.Hour)
	if _, err := o.Query(context.Background(), openssl12); err != nil || fake.requests.Load() != before+1 {
		t.Errorf("Debian lookup at 25 hours should query again; err %v", err)
	}
}
