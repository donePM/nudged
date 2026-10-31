package daemon

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/donepm/nudged/internal/proto"
)

func fixture(t *testing.T, event string) string {
	t.Helper()
	b, err := os.ReadFile("../proto/testdata/hooks/" + event + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// noticeHook sends one hook with a TTY and a Term and checks that the reply is OK with no output.
func noticeHook(t *testing.T, path, event, payload string) {
	t.Helper()
	rep := send(t, path, proto.Request{Kind: proto.Hook, Event: event, Payload: json.RawMessage(payload), Term: "iTerm.app", TTY: "/dev/ttys001"})
	if b, _ := json.Marshal(rep); string(b) != `{"ok":true}` {
		t.Fatalf("%s: reply %s", event, b)
	}
}

func TestStopNotice(t *testing.T) {
	path := sockPath(t)
	srv := start(t, path, NewRegistry())
	noticeHook(t, path, "Stop", fixture(t, "Stop"))
	list := srv.Pending()
	if len(list) != 1 {
		t.Fatalf("pending %+v", list)
	}
	if p := list[0]; p.Kind != proto.Done || p.SessionID != "00000000-0000-4000-8000-000000000001" || p.TTY != "/dev/ttys001" || p.Term != "iTerm.app" {
		t.Fatalf("pending %+v", p)
	}

	noticeHook(t, path, "Stop", `{"session_id":"s2","background_tasks":[{"type":"subagent","status":"running"}]}`)
	if list := srv.Pending(); len(list) != 1 || list[0].SessionID == "s2" {
		t.Fatalf("background tasks: pending %+v", list)
	}
}

func TestWaitsNotice(t *testing.T) {
	path := sockPath(t)
	srv := start(t, path, NewRegistry())
	noticeHook(t, path, "Notification", `{"session_id":"s1","notification_type":"permission_prompt"}`)
	if list := srv.Pending(); len(list) != 0 {
		t.Fatalf("other type: pending %+v", list)
	}
	noticeHook(t, path, "Notification", fixture(t, "Notification"))
	if list := srv.Pending(); len(list) != 1 || list[0].Kind != proto.Waits {
		t.Fatalf("pending %+v", list)
	}

	srv.SetPaused(true)
	noticeHook(t, path, "Notification", fixture(t, "Notification"))
	noticeHook(t, path, "Stop", fixture(t, "Stop"))
	if list := srv.Pending(); len(list) != 0 {
		t.Fatalf("paused: pending %+v", list)
	}
}

func TestDismiss(t *testing.T) {
	path := sockPath(t)
	srv := start(t, path, NewRegistry())
	noticeHook(t, path, "Stop", `{"session_id":"s1"}`)
	waitSnap(t, srv, "1")
	if err := srv.Dismiss("1"); err != nil {
		t.Fatal(err)
	}
	waitSnap(t, srv)
	if err := srv.Dismiss("1"); err == nil {
		t.Fatal("second dismiss: no error")
	}
	if err := srv.Dismiss("99"); err == nil {
		t.Fatal("unknown id: no error")
	}

	noticeHook(t, path, "Stop", `{"session_id":"s1"}`)
	waitSnap(t, srv, "2")
	hook(t, path, "UserPromptSubmit", `{"session_id":"s1"}`)
	waitSnap(t, srv)
}

func TestWaitsReplacesOlder(t *testing.T) {
	path := sockPath(t)
	srv := start(t, path, NewRegistry())
	idle := func(session string) string {
		return `{"session_id":"` + session + `","notification_type":"idle_prompt"}`
	}
	noticeHook(t, path, "Stop", `{"session_id":"s1"}`)
	noticeHook(t, path, "Notification", idle("s1"))
	noticeHook(t, path, "Notification", idle("s2"))
	noticeHook(t, path, "Notification", idle("s1"))
	var got [][3]string
	for _, p := range srv.Pending() {
		got = append(got, [3]string{p.ID, p.Kind, p.SessionID})
	}
	want := [][3]string{{"1", proto.Done, "s1"}, {"3", proto.Waits, "s2"}, {"4", proto.Waits, "s1"}}
	if !slices.Equal(got, want) {
		t.Fatalf("pending %v, want %v", got, want)
	}
}
