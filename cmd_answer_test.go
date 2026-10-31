package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rokde/nudged/internal/daemon"
	"github.com/rokde/nudged/internal/proto"
)

func startDaemon(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "nd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "s.sock")
	srv, err := daemon.Listen(socket, daemon.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve()
	t.Cleanup(func() { srv.Close() })
	return socket
}

// holdPermission sends one PermissionRequest hook and keeps it open. The channel gets the reply line.
func holdPermission(t *testing.T, socket string) chan string {
	t.Helper()
	conn, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	payload := json.RawMessage(`{"session_id":"s1","cwd":"/tmp/proj","tool_name":"Bash"}`)
	if err := proto.WriteLine(conn, proto.Request{Kind: proto.Hook, Event: "PermissionRequest", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	ch := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(conn).ReadString('\n')
		ch <- strings.TrimSuffix(line, "\n")
	}()
	return ch
}

func waitHeld(t *testing.T, socket string, n int) []proto.PendingRequest {
	t.Helper()
	for end := time.Now().Add(2 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		var held []proto.PendingRequest
		if err := list(socket, proto.Pending, &held); err != nil {
			t.Fatal(err)
		}
		if len(held) == n {
			return held
		}
		if time.Now().After(end) {
			t.Fatalf("pending %+v, want %d", held, n)
		}
	}
}

func TestAnswerHeld(t *testing.T) {
	socket := startDaemon(t)
	ch := holdPermission(t, socket)
	id := waitHeld(t, socket, 1)[0].ID
	payload := `{"hookSpecificOutput":{"hookEventName":"PermissionRequest","decision":{"behavior":"allow"}}}`
	var out, errOut bytes.Buffer
	if code := runAnswer(socket, []string{id, payload}, &out, &errOut); code != 0 || out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("code %d, stdout %q, stderr %q", code, out.String(), errOut.String())
	}
	select {
	case got := <-ch:
		if want := `{"ok":true,"output":` + payload + `}`; got != want {
			t.Fatalf("hook reply %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no hook reply")
	}
	waitHeld(t, socket, 0)
}

func TestAnswerErrors(t *testing.T) {
	socket := startDaemon(t)
	var out, errOut bytes.Buffer
	if code := runAnswer(socket, []string{"99", `{}`}, &out, &errOut); code != 1 || out.Len() != 0 ||
		!strings.HasPrefix(errOut.String(), "nudged answer:") || !strings.Contains(errOut.String(), `unknown id "99"`) {
		t.Fatalf("unknown id: code %d, stdout %q, stderr %q", code, out.String(), errOut.String())
	}

	// No daemon listens here: a dial would print "daemon not running".
	none := filepath.Join(filepath.Dir(socket), "none.sock")
	for name, args := range map[string][]string{
		"bad json": {"1", "{bad"},
		"no args":  nil,
		"one arg":  {"1"},
		"3 args":   {"1", "{}", "x"},
	} {
		out.Reset()
		errOut.Reset()
		if code := runAnswer(none, args, &out, &errOut); code != 1 || out.Len() != 0 ||
			!strings.HasPrefix(errOut.String(), "nudged answer:") || strings.Contains(errOut.String(), "daemon not running") {
			t.Errorf("%s: code %d, stdout %q, stderr %q", name, code, out.String(), errOut.String())
		}
	}
}

func TestAnswerNoDaemon(t *testing.T) {
	dir, err := os.MkdirTemp("", "nd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	var out, errOut bytes.Buffer
	if code := runAnswer(filepath.Join(dir, "none.sock"), []string{"1", `{}`}, &out, &errOut); code != 1 || out.Len() != 0 {
		t.Fatalf("code %d, stdout %q", code, out.String())
	}
	if errOut.String() != "nudged answer: daemon not running\n" {
		t.Fatalf("stderr %q", errOut.String())
	}
}
