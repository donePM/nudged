package app

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/donepm/nudged/internal/daemon"
	"github.com/donepm/nudged/internal/proto"
	"github.com/donepm/nudged/internal/usage"
)

func TestWaitingCountAndBadge(t *testing.T) {
	list := []proto.Session{{Status: "waiting"}, {Status: "working"}, {Status: "waiting"}, {Status: "done"}, {Status: "idle"}}
	if n := waitingCount(list); n != 2 {
		t.Errorf("waitingCount = %d, want 2", n)
	}
	if n := waitingCount(nil); n != 0 {
		t.Errorf("waitingCount(nil) = %d, want 0", n)
	}
	if l := badgeLabel(0); l != "" {
		t.Errorf("badgeLabel(0) = %q, want empty", l)
	}
	if l := badgeLabel(3); l != "3" {
		t.Errorf("badgeLabel(3) = %q, want 3", l)
	}
}

func TestBuildPayload(t *testing.T) {
	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	list := []proto.Session{
		{ID: "b", Project: "p", Path: "/x/p", Status: "working", Term: "iTerm.app", TTY: "/dev/ttys001", Model: "opus", Effort: "high", ContextPct: 42, LastActivity: at},
		{ID: "a", Status: "waiting"},
	}
	p := buildPayload(list, usage.Totals{InputTokens: 1, OutputTokens: 20, CacheCreationTokens: 300, CacheReadTokens: 4000, CostUSD: 1.5})
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Sessions []map[string]any `json:"sessions"`
		Usage    map[string]any   `json:"usage"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Sessions) != 2 || got.Sessions[0]["session_id"] != "b" || got.Sessions[1]["session_id"] != "a" {
		t.Fatalf("order not kept: %s", b)
	}
	want := map[string]any{
		"project": "p", "path": "/x/p", "status": "working", "term": "iTerm.app", "model": "opus",
		"effort": "high", "context_pct": float64(42), "session_id": "b", "last_activity": "2026-10-31T12:00:00Z",
	}
	for k, v := range want {
		if got.Sessions[0][k] != v {
			t.Errorf("%s = %v, want %v", k, got.Sessions[0][k], v)
		}
	}
	if _, ok := got.Sessions[0]["tty"]; ok {
		t.Error("payload has tty")
	}
	if got.Usage["tokens"] != float64(4321) || got.Usage["cost_usd"] != 1.5 {
		t.Errorf("usage = %v", got.Usage)
	}
}

func TestBuildPayloadEmpty(t *testing.T) {
	b, err := json.Marshal(buildPayload(nil, usage.Totals{}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"sessions":[]`) {
		t.Errorf("got %s, want sessions []", b)
	}
}

func TestFocusSession(t *testing.T) {
	reg := daemon.NewRegistry()
	reg.Hook("SessionStart", []byte(`{"session_id":"s1","cwd":"/x/a"}`), "Apple_Terminal", "/dev/ttys004")
	reg.Hook("SessionStart", []byte(`{"session_id":"s2","cwd":"/x/b"}`), "iTerm.app", "/dev/ttys009")
	type call struct{ tty, term string }
	var calls []call
	jump := func(tty, term string) { calls = append(calls, call{tty, term}) }

	focusSession(reg, "s2", jump)
	if len(calls) != 1 || calls[0] != (call{"/dev/ttys009", "iTerm.app"}) {
		t.Fatalf("calls = %v", calls)
	}
	calls = nil
	focusSession(reg, "nope", jump)
	if len(calls) != 0 {
		t.Errorf("unknown id calls = %v", calls)
	}
}
