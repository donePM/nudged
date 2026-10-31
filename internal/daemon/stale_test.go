package daemon

import (
	"testing"
	"time"

	"github.com/donepm/nudged/internal/proto"
)

func staleReg(clock *time.Time) *Registry {
	reg := NewRegistry()
	reg.now = func() time.Time { return *clock }
	return reg
}

func ids(l []proto.Session) []string {
	var out []string
	for _, s := range l {
		out = append(out, s.ID)
	}
	return out
}

func TestListDropsStale(t *testing.T) { // AC-1, AC-2
	clock := time.Unix(1000000, 0)
	reg := staleReg(&clock)
	reg.Hook("SessionStart", []byte(`{"session_id":"a"}`), "", "")
	clock = clock.Add(staleAfter)
	if got := reg.List(); len(got) != 1 {
		t.Fatalf("exactly 12h must stay, got %v", ids(got))
	}
	clock = clock.Add(time.Second)
	if got := reg.List(); len(got) != 0 {
		t.Fatalf("stale session returned: %v", ids(got))
	}
}

func TestActivityRevivesSession(t *testing.T) { // AC-3
	clock := time.Unix(1000000, 0)
	reg := staleReg(&clock)
	reg.Hook("SessionStart", []byte(`{"session_id":"a"}`), "", "")
	reg.Hook("SessionStart", []byte(`{"session_id":"b"}`), "", "")
	clock = clock.Add(11 * time.Hour)
	reg.Hook("PostToolUse", []byte(`{"session_id":"a"}`), "", "")
	reg.Statusline(proto.StatuslineInfo{SessionID: "b"})
	clock = clock.Add(2 * time.Hour)
	if got := reg.List(); len(got) != 2 {
		t.Fatalf("want 2 sessions, got %v", ids(got))
	}
}

func TestStaleStaysGone(t *testing.T) { // AC-4
	clock := time.Unix(1000000, 0)
	reg := staleReg(&clock)
	reg.Hook("SessionStart", []byte(`{"session_id":"a"}`), "", "")
	clock = clock.Add(13 * time.Hour)
	reg.List()
	if len(reg.sessions) != 0 {
		t.Fatal("stale session still in map")
	}
	clock = clock.Add(-13 * time.Hour)
	if got := reg.List(); len(got) != 0 {
		t.Fatalf("session came back: %v", ids(got))
	}
}

func TestStaleKeepsOrder(t *testing.T) { // AC-5
	clock := time.Unix(1000000, 0)
	reg := staleReg(&clock)
	reg.Hook("SessionStart", []byte(`{"session_id":"old"}`), "", "")
	clock = clock.Add(5 * time.Hour)
	reg.Hook("SessionStart", []byte(`{"session_id":"x"}`), "", "")
	clock = clock.Add(time.Hour)
	reg.Hook("SessionStart", []byte(`{"session_id":"y"}`), "", "")
	clock = clock.Add(time.Hour)
	reg.Hook("Notification", []byte(`{"session_id":"x"}`), "", "")
	clock = clock.Add(6 * time.Hour)
	got := ids(reg.List())
	if len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Fatalf("got %v", got)
	}
}
