package daemon

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rokde/nudged/internal/proto"
)

func sockPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "nd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "cfg", "s.sock")
}

func start(t *testing.T, path string, reg *Registry) *Server {
	t.Helper()
	srv, err := Listen(path, reg)
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve()
	t.Cleanup(func() { srv.Close() })
	return srv
}

func send(t *testing.T, path string, req proto.Request) proto.Reply {
	t.Helper()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := proto.WriteLine(conn, req); err != nil {
		t.Fatal(err)
	}
	var rep proto.Reply
	if err := proto.ReadLine(bufio.NewReader(conn), &rep); err != nil {
		t.Fatal(err)
	}
	return rep
}

func hook(t *testing.T, path, event, payload string) {
	t.Helper()
	rep := send(t, path, proto.Request{Kind: proto.Hook, Event: event, Payload: json.RawMessage(payload)})
	if !rep.OK || rep.Output != nil {
		t.Fatalf("%s: reply %+v", event, rep)
	}
}

func status(t *testing.T, path string) []proto.Session {
	t.Helper()
	rep := send(t, path, proto.Request{Kind: proto.Status})
	if !rep.OK {
		t.Fatalf("reply %+v", rep)
	}
	var list []proto.Session
	if err := json.Unmarshal(rep.Output, &list); err != nil {
		t.Fatal(err)
	}
	return list
}

func TestSocketLifecycle(t *testing.T) {
	path := sockPath(t)
	srv, err := Listen(path, NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != proto.SocketMode {
		t.Fatalf("socket mode %o", fi.Mode().Perm())
	}
	if di, _ := os.Stat(filepath.Dir(path)); di.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode %o", di.Mode().Perm())
	}
	if _, err := Listen(path, NewRegistry()); err == nil {
		t.Fatal("second start did not fail")
	}
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("socket file still exists: %v", err)
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close()
	start(t, path, NewRegistry())
	if got := status(t, path); len(got) != 0 {
		t.Fatalf("sessions %+v", got)
	}
}

func TestNotSocketKept(t *testing.T) {
	path := sockPath(t)
	os.MkdirAll(filepath.Dir(path), 0o700)
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Listen(path, NewRegistry()); err == nil {
		t.Fatal("start on a plain file did not fail")
	}
	if data, _ := os.ReadFile(path); string(data) != "x" {
		t.Fatal("plain file changed")
	}
}

func TestSessionStart(t *testing.T) {
	path := sockPath(t)
	start(t, path, NewRegistry())
	rep := send(t, path, proto.Request{Kind: proto.Hook, Event: "SessionStart",
		Payload: json.RawMessage(`{"session_id":"s1","cwd":"/tmp/proj-a"}`), Term: "iTerm.app", TTY: "/dev/ttys003"})
	if b, _ := json.Marshal(rep); string(b) != `{"ok":true}` {
		t.Fatalf("reply %s", b)
	}
	list := status(t, path)
	if len(list) != 1 {
		t.Fatalf("sessions %+v", list)
	}
	s := list[0]
	if s.ID != "s1" || s.Project != "proj-a" || s.Path != "/tmp/proj-a" || s.Status != "idle" || s.Term != "iTerm.app" || s.TTY != "/dev/ttys003" {
		t.Fatalf("session %+v", s)
	}
}

func TestHookEvents(t *testing.T) {
	path := sockPath(t)
	start(t, path, NewRegistry())
	p := `{"session_id":"s1","cwd":"/tmp/p"}`
	for _, c := range [][2]string{{"UserPromptSubmit", "working"}, {"PostToolUse", "working"}, {"Notification", "waiting"}, {"Stop", "done"}} {
		hook(t, path, c[0], p)
		if list := status(t, path); len(list) != 1 || list[0].Status != c[1] {
			t.Fatalf("%s: sessions %+v", c[0], list)
		}
	}
	before := status(t, path)
	for name, c := range map[string][2]string{
		"unknown event": {"PreCompact", p},
		"no session_id": {"UserPromptSubmit", `{"cwd":"/tmp/x"}`},
		"not an object": {"UserPromptSubmit", `"s1"`},
	} {
		hook(t, path, c[0], c[1])
		if after := status(t, path); len(after) != 1 || after[0] != before[0] {
			t.Fatalf("%s: sessions %+v", name, after)
		}
	}
	hook(t, path, "SessionEnd", p)
	if list := status(t, path); len(list) != 0 {
		t.Fatalf("sessions %+v", list)
	}
}

func TestStatusline(t *testing.T) {
	path := sockPath(t)
	start(t, path, NewRegistry())
	hook(t, path, "Notification", `{"session_id":"s1","cwd":"/tmp/p"}`)
	for _, id := range []string{"s1", "s2"} {
		info, _ := json.Marshal(proto.StatuslineInfo{SessionID: id, Cwd: "/tmp/p", Model: "Opus", Effort: "high", ContextPct: 42})
		if rep := send(t, path, proto.Request{Kind: proto.Statusline, Payload: info}); !rep.OK || rep.Output != nil {
			t.Fatalf("reply %+v", rep)
		}
	}
	list := status(t, path)
	want := map[string]string{"s1": "waiting", "s2": "idle"}
	if len(list) != 2 {
		t.Fatalf("sessions %+v", list)
	}
	for _, s := range list {
		if s.Model != "Opus" || s.Effort != "high" || s.ContextPct != 42 || s.Status != want[s.ID] {
			t.Fatalf("session %+v", s)
		}
	}
}

func TestSortOrder(t *testing.T) {
	reg := NewRegistry()
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	reg.now = func() time.Time { return clock }
	path := sockPath(t)
	start(t, path, reg)
	for _, c := range [][2]string{{"B", "Notification"}, {"C", "UserPromptSubmit"}, {"A", "Stop"}} {
		clock = clock.Add(time.Minute)
		hook(t, path, c[1], `{"session_id":"`+c[0]+`"}`)
	}
	list := status(t, path)
	var ids string
	for _, s := range list {
		ids += s.ID
	}
	if ids != "BAC" {
		t.Fatalf("order %q", ids)
	}
	if b, _ := json.Marshal(list[0]); !json.Valid(b) || list[0].LastActivity != clock.Add(-2*time.Minute) {
		t.Fatalf("session %s", b)
	}
}

func TestPromptStarted(t *testing.T) {
	reg := NewRegistry()
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	reg.now = func() time.Time { return clock }
	p := []byte(`{"session_id":"s1"}`)
	reg.Hook("UserPromptSubmit", p, "", "")
	prompt := clock
	for _, e := range []string{"PostToolUse", "Stop"} {
		clock = clock.Add(time.Minute)
		reg.Hook(e, p, "", "")
		if list := reg.List(); len(list) != 1 || !list[0].PromptStarted.Equal(prompt) {
			t.Fatalf("%s: sessions %+v", e, list)
		}
	}
}
