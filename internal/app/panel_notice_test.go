package app

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/donepm/nudged/internal/proto"
)

func noticeRequest(kind string) proto.PendingRequest {
	return proto.PendingRequest{
		ID: "n1", Kind: kind, SessionID: "s1", Payload: json.RawMessage(`{"session_id":"s1","cwd":"/work/fromcwd"}`),
		Started: time.Date(2026, 10, 31, 14, 3, 0, 0, time.UTC),
	}
}

func TestNoticeView(t *testing.T) {
	for _, k := range []string{proto.Done, proto.Waits} {
		if v := viewFor(k); v != "Notice" {
			t.Fatalf("%s: view %q", k, v)
		}
	}
	for _, mode := range []string{"answer", "notify"} {
		if v := panelView(&panelShow{Request: noticeRequest(proto.Done), Mode: mode}); v != "Notice" {
			t.Fatalf("%s: view %q", mode, v)
		}
	}
	if _, err := os.Stat("../../frontend/src/views/Notice.vue"); err != nil {
		t.Fatal(err)
	}
}

func TestNoticePanelDone(t *testing.T) {
	req := noticeRequest(proto.Done)
	sessions := []proto.Session{
		{ID: "other", Project: "x", PromptStarted: req.Started.Add(-time.Hour)},
		{ID: "s1", Project: "proj", PromptStarted: req.Started.Add(-125*time.Second - 300*time.Millisecond)},
	}
	d := noticePanel(req, sessions)
	if d.Title != "Session done" || d.Project != "proj" || d.Session != "s1" || d.Duration != 125 || !d.Since.Equal(req.Started) {
		t.Fatalf("done: %+v", d)
	}
	if d := noticePanel(req, nil); d.Project != "fromcwd" || d.Duration != 0 {
		t.Fatalf("no session: %+v", d)
	}
	if d := noticePanel(req, []proto.Session{{ID: "s1", Project: "proj"}}); d.Duration != 0 {
		t.Fatalf("no prompt: %+v", d)
	}
	if d := noticePanel(req, []proto.Session{{ID: "s1", PromptStarted: req.Started.Add(time.Minute)}}); d.Duration != 0 {
		t.Fatalf("negative: %+v", d)
	}
}

func TestNoticePanelWaits(t *testing.T) {
	req := noticeRequest(proto.Waits)
	d := noticePanel(req, []proto.Session{{ID: "s1", Project: "proj", PromptStarted: req.Started.Add(-time.Minute)}})
	if d.Title != "Session waits" || d.Project != "proj" || d.Session != "s1" || !d.Since.Equal(req.Started) || d.Duration != 0 {
		t.Fatalf("waits: %+v", d)
	}
}
