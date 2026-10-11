package main

import (
	"context"
	"testing"
)

func TestUC8_EndOfLifeNote(t *testing.T) {
	e, _ := newFakeEndOfLife(t)
	ctx := context.Background()
	want := &EndOfLifeNote{Product: "Node.js", Cycle: "20", Date: "2026-04-30"}

	// UC-8.1: the leading version of the tag picks the cycle.
	for _, tag := range []string{"20", "20-alpine", "20.11.1-bookworm-slim", "v20"} {
		if got := e.Note(ctx, "library/node", tag); got == nil || *got != *want {
			t.Errorf("UC-8.1 node:%s: got %+v, want %+v", tag, got, want)
		}
	}
	// UC-8.2: no confident mapping, no note.
	for _, c := range [][2]string{
		{"library/node", "latest"}, {"library/node", "lts"}, {"library/node", "bookworm"},
		{"library/unknown-app", "1.0"},
		{"bitnami/node", "20"}, // not the official image
		{"library/node", "19"}, // a cycle endoflife.date doesn't list
	} {
		if got := e.Note(ctx, c[0], c[1]); got != nil {
			t.Errorf("UC-8.2 %s:%s: got %+v, want no note", c[0], c[1], got)
		}
	}
	// Releases still in support, and one whose end of life has no date.
	for _, tag := range []string{"22", "24", "17"} {
		if got := e.Note(ctx, "library/node", tag); got != nil {
			t.Errorf("node:%s: got %+v, want no note", tag, got)
		}
	}
	if got := e.Note(ctx, "library/python", "3.12-slim"); got != nil {
		t.Errorf("python:3.12-slim is supported until 2028: %+v", got)
	}
}

func TestUC11_EndOfLifeFailureOmitsNote(t *testing.T) {
	e, fake := newFakeEndOfLife(t)
	fake.fail.Store(true)
	if got := e.Note(context.Background(), "library/node", "20"); got != nil {
		t.Errorf("a failed lookup should give no note, got %+v", got)
	}
}

func TestUC10_EndOfLifeCached(t *testing.T) {
	e, fake := newFakeEndOfLife(t)
	e.Note(context.Background(), "library/node", "20")
	e.Note(context.Background(), "library/node", "18")
	if n := fake.requests.Load(); n != 1 {
		t.Errorf("requests = %d, want 1 (cached for 24 hours)", n)
	}
}
