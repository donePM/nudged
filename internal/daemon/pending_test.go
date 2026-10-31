package daemon

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/rokde/nudged/internal/hookclient"
	"github.com/rokde/nudged/internal/proto"
)

// holdHook sends one hook and keeps the connection open. The channel gets the reply line, or "" when the read fails.
func holdHook(t *testing.T, path, event, payload string) (net.Conn, chan string) {
	t.Helper()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := proto.WriteLine(conn, proto.Request{Kind: proto.Hook, Event: event, Payload: json.RawMessage(payload), Term: "iTerm.app", TTY: "/dev/ttys001"}); err != nil {
		t.Fatal(err)
	}
	ch := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(conn).ReadString('\n')
		ch <- strings.TrimSuffix(line, "\n")
	}()
	return conn, ch
}

func pending(t *testing.T, path string) []proto.PendingRequest {
	t.Helper()
	rep := send(t, path, proto.Request{Kind: proto.Pending})
	if !rep.OK {
		t.Fatalf("reply %+v", rep)
	}
	var list []proto.PendingRequest
	if err := json.Unmarshal(rep.Output, &list); err != nil {
		t.Fatal(err)
	}
	return list
}

// waitPending waits until the queue has n requests, because the server adds a request after the client sent it.
func waitPending(t *testing.T, path string, n int) []proto.PendingRequest {
	t.Helper()
	for end := time.Now().Add(2 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		list := pending(t, path)
		if len(list) == n {
			return list
		}
		if time.Now().After(end) {
			t.Fatalf("pending %+v, want %d", list, n)
		}
	}
}

func noReply(t *testing.T, ch chan string, d time.Duration) {
	t.Helper()
	select {
	case got := <-ch:
		t.Fatalf("got reply %q", got)
	case <-time.After(d):
	}
}

func reply(t *testing.T, ch chan string, want string) {
	t.Helper()
	select {
	case got := <-ch:
		if got != want {
			t.Fatalf("reply %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no reply")
	}
}

func startWith(t *testing.T, path string, hold, line time.Duration) *Server {
	t.Helper()
	srv, err := Listen(path, NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	srv.HoldTimeout, srv.LineDeadline = hold, line
	go srv.Serve()
	t.Cleanup(func() { srv.Close() })
	return srv
}

const (
	perm1 = `{"session_id":"s1","cwd":"/tmp/p","tool_name":"Bash"}`
	ask2  = `{"session_id":"s2","tool_name":"AskUserQuestion"}`
)

func TestHold(t *testing.T) {
	path := sockPath(t)
	start(t, path, NewRegistry())
	_, ch1 := holdHook(t, path, "PermissionRequest", perm1)
	waitPending(t, path, 1)
	_, ch2 := holdHook(t, path, "PermissionRequest", ask2)
	list := waitPending(t, path, 2)
	hook(t, path, "PreToolUse", `{"session_id":"s3","tool_name":"Bash"}`)
	noReply(t, ch1, 50*time.Millisecond)
	noReply(t, ch2, 0)

	want := [][4]string{{"1", proto.Permission, "s1", perm1}, {"2", proto.Question, "s2", ask2}}
	for i, p := range list {
		got := [4]string{p.ID, p.Kind, p.SessionID, string(p.Payload)}
		if got != want[i] || p.Term != "iTerm.app" || p.TTY != "/dev/ttys001" || p.Started.IsZero() {
			t.Fatalf("pending[%d] %+v", i, p)
		}
	}
	if !list[0].Started.Before(list[1].Started) {
		t.Fatal("not oldest first")
	}
	for _, s := range status(t, path) {
		if want := map[string]string{"s1": "waiting", "s2": "waiting", "s3": "idle"}[s.ID]; s.Status != want {
			t.Fatalf("session %+v, want %s", s, want)
		}
	}
}

func TestPreToolUseNotHeld(t *testing.T) {
	path := sockPath(t)
	start(t, path, NewRegistry())
	rep := send(t, path, proto.Request{Kind: proto.Hook, Event: "PreToolUse", Payload: json.RawMessage(ask2)})
	if b, _ := json.Marshal(rep); string(b) != `{"ok":true}` {
		t.Fatalf("reply %s", b)
	}
	if list := pending(t, path); len(list) != 0 {
		t.Fatalf("pending %+v", list)
	}
}

func TestAnswer(t *testing.T) {
	path := sockPath(t)
	start(t, path, NewRegistry())
	_, ch := holdHook(t, path, "PermissionRequest", perm1)
	waitPending(t, path, 1)

	for name, req := range map[string]proto.Request{
		"unknown id": {Kind: proto.Answer, ID: "99", Payload: json.RawMessage(`{}`)},
		"no payload": {Kind: proto.Answer, ID: "1"},
	} {
		if rep := send(t, path, req); rep.OK || rep.Error == "" {
			t.Fatalf("%s: reply %+v", name, rep)
		}
	}
	bad, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer bad.Close()
	bad.Write([]byte(`{"kind":"answer","id":"1","payload":{bad}}` + "\n"))
	var rep proto.Reply
	if err := proto.ReadLine(bufio.NewReader(bad), &rep); err != nil || rep.OK || rep.Error == "" {
		t.Fatalf("bad payload: reply %+v, err %v", rep, err)
	}
	waitPending(t, path, 1)
	noReply(t, ch, 0)

	rep = send(t, path, proto.Request{Kind: proto.Answer, ID: "1", Payload: json.RawMessage(`{"decision":"allow"}`)})
	if b, _ := json.Marshal(rep); string(b) != `{"ok":true}` {
		t.Fatalf("reply %s", b)
	}
	reply(t, ch, `{"ok":true,"output":{"decision":"allow"}}`)
	waitPending(t, path, 0)
	if rep := send(t, path, proto.Request{Kind: proto.Answer, ID: "1", Payload: json.RawMessage(`{}`)}); rep.OK {
		t.Fatal("second answer accepted")
	}
}

func TestHoldTimeout(t *testing.T) {
	path := sockPath(t)
	startWith(t, path, 300*time.Millisecond, 50*time.Millisecond)
	_, ch := holdHook(t, path, "PermissionRequest", perm1)
	noReply(t, ch, 150*time.Millisecond)
	reply(t, ch, `{"ok":true}`)
	waitPending(t, path, 0)
}

func TestEndRequest(t *testing.T) {
	path := sockPath(t)
	start(t, path, NewRegistry())
	conn, _ := holdHook(t, path, "PermissionRequest", perm1)
	waitPending(t, path, 1)
	conn.Close()
	waitPending(t, path, 0)

	_, ch := holdHook(t, path, "PermissionRequest", perm1)
	waitPending(t, path, 1)
	hook(t, path, "Notification", `{"session_id":"s1"}`)
	info, _ := json.Marshal(proto.StatuslineInfo{SessionID: "s1", Model: "Opus"})
	send(t, path, proto.Request{Kind: proto.Statusline, Payload: info})
	hook(t, path, "PostToolUse", `{"session_id":"s2"}`)
	waitPending(t, path, 1)
	noReply(t, ch, 50*time.Millisecond)

	hook(t, path, "PostToolUse", `{"session_id":"s1"}`)
	reply(t, ch, `{"ok":true}`)
	waitPending(t, path, 0)
}

func TestPause(t *testing.T) {
	path := sockPath(t)
	srv := start(t, path, NewRegistry())
	_, ch1 := holdHook(t, path, "PermissionRequest", perm1)
	_, ch2 := holdHook(t, path, "PermissionRequest", ask2)
	waitPending(t, path, 2)
	srv.SetPaused(true)
	reply(t, ch1, `{"ok":true}`)
	reply(t, ch2, `{"ok":true}`)
	waitPending(t, path, 0)
	hook(t, path, "PermissionRequest", perm1)
	waitPending(t, path, 0)

	srv.SetPaused(false)
	_, ch := holdHook(t, path, "PermissionRequest", perm1)
	waitPending(t, path, 1)
	noReply(t, ch, 50*time.Millisecond)
}

func TestHookClientEndToEnd(t *testing.T) {
	path := sockPath(t)
	startWith(t, path, 300*time.Millisecond, 0)
	answer := `{"hookSpecificOutput":{"hookEventName":"PermissionRequest","decision":{"behavior":"allow"}}}`

	run := func() chan [2]string {
		done := make(chan [2]string, 1)
		go func() {
			var out bytes.Buffer
			code := hookclient.Run("PermissionRequest", strings.NewReader(perm1), &out, path, "iTerm.app")
			done <- [2]string{string(rune('0' + code)), out.String()}
		}()
		return done
	}

	done := run()
	list := waitPending(t, path, 1)
	if rep := send(t, path, proto.Request{Kind: proto.Answer, ID: list[0].ID, Payload: json.RawMessage(answer)}); !rep.OK {
		t.Fatalf("reply %+v", rep)
	}
	if got := <-done; got != [2]string{"0", answer + "\n"} {
		t.Fatalf("code, stdout %q", got)
	}

	done = run()
	waitPending(t, path, 1)
	if got := <-done; got != [2]string{"0", ""} {
		t.Fatalf("code, stdout %q", got)
	}
	waitPending(t, path, 0)
}
