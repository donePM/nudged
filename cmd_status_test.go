package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rokde/nudged/internal/proto"
)

func TestPrintSessions(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 10, 0, 0, time.UTC)
	sessions := []proto.Session{
		{ID: "B", Project: "pb", Path: "/tmp/pb", Status: "waiting", Model: "Opus", Effort: "high", ContextPct: 42, Term: "iTerm.app", TTY: "/dev/ttys003", LastActivity: now.Add(-3 * time.Minute)},
		{ID: "A", Project: "pa", Path: "/tmp/pa", Status: "done", LastActivity: now.Add(-time.Minute)},
		{ID: "C", Project: "pc", Path: "/tmp/pc", Status: "working", LastActivity: now.Add(-2 * time.Minute)},
	}
	var buf bytes.Buffer
	printSessions(&buf, sessions, now)
	rows := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(rows) != 3 {
		t.Fatalf("rows %q", rows)
	}
	for i, want := range []string{"pb", "pa", "pc"} {
		if !strings.Contains(rows[i], want) {
			t.Errorf("row %d %q has no %q", i, rows[i], want)
		}
	}
	for _, want := range []string{"waiting", "Opus", "high", "42%", "iTerm.app", "/dev/ttys003", "3m0s", "/tmp/pb"} {
		if !strings.Contains(rows[0], want) {
			t.Errorf("row %q has no %q", rows[0], want)
		}
	}
	buf.Reset()
	printSessions(&buf, nil, now)
	if buf.String() != "no sessions\n" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestStatusNoDaemon(t *testing.T) {
	dir, err := os.MkdirTemp("", "nd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	var out, errOut bytes.Buffer
	if code := runStatus(filepath.Join(dir, "none.sock"), &out, &errOut); code != 1 || out.Len() != 0 {
		t.Fatalf("code %d, stdout %q", code, out.String())
	}
	if errOut.String() != "nudged status: daemon not running\n" {
		t.Fatalf("stderr %q", errOut.String())
	}
}

func TestCommandsRegistered(t *testing.T) {
	for _, name := range []string{"daemon", "status"} {
		if _, ok := commands[name]; !ok {
			t.Errorf("%s not registered", name)
		}
	}
}
