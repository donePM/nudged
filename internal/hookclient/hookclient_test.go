package hookclient

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

	"github.com/donepm/nudged/internal/proto"
)

// fakeServer accepts connections on a short socket path. For each one it sends the received line on the channel, then writes reply (if not empty) and closes.
func fakeServer(t *testing.T, reply string) (string, chan string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "nd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "s.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	lines := make(chan string, 10)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			line, _ := bufio.NewReader(conn).ReadString('\n')
			lines <- line
			if reply != "" {
				conn.Write([]byte(reply + "\n"))
			}
			conn.Close()
		}
	}()
	return sock, lines
}

func TestForwardAndPrint(t *testing.T) {
	sock, lines := fakeServer(t, `{"ok":true,"output":{"decision":"allow"}}`)
	var out bytes.Buffer
	if code := Run("PermissionRequest", strings.NewReader(`{"tool_name":"Bash"}`), &out, sock, "iTerm.app"); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if got := out.String(); got != "{\"decision\":\"allow\"}\n" {
		t.Fatalf("stdout %q", got)
	}
	line := <-lines
	if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
		t.Fatalf("not one line: %q", line)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{"kind": `"hook"`, "event": `"PermissionRequest"`, "payload": `{"tool_name":"Bash"}`, "term": `"iTerm.app"`} {
		if string(got[k]) != want {
			t.Errorf("%s = %s, want %s", k, got[k], want)
		}
	}
}

func TestNoDaemon(t *testing.T) {
	dir, err := os.MkdirTemp("", "nd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	stale := filepath.Join(dir, "stale.sock")
	ln, err := net.Listen("unix", stale)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close()
	for name, sock := range map[string]string{"missing": filepath.Join(dir, "none.sock"), "no listener": stale} {
		var out bytes.Buffer
		start := time.Now()
		code := Run("PermissionRequest", strings.NewReader(`{}`), &out, sock, "")
		if d := time.Since(start); code != 0 || out.Len() != 0 || d >= time.Second {
			t.Errorf("%s: code %d, stdout %q, took %v", name, code, out.String(), d)
		}
	}
}

func TestBadReply(t *testing.T) {
	for name, reply := range map[string]string{
		"not ok":    `{"ok":false,"output":{"decision":"allow"}}`,
		"no output": `{"ok":true}`,
		"null":      `{"ok":true,"output":null}`,
		"no reply":  "",
		"not json":  "nope",
	} {
		sock, lines := fakeServer(t, reply)
		var out bytes.Buffer
		if code := Run("Stop", strings.NewReader(`{}`), &out, sock, ""); code != 0 || out.Len() != 0 {
			t.Errorf("%s: code %d, stdout %q", name, code, out.String())
		}
		<-lines
	}
}

func TestBadInput(t *testing.T) {
	sock, lines := fakeServer(t, `{"ok":true,"output":{}}`)
	for name, c := range map[string][2]string{
		"bad json": {"PermissionRequest", `{"tool_name":`},
		"empty":    {"PermissionRequest", ``},
		"no event": {"", `{}`},
	} {
		var out bytes.Buffer
		if code := Run(c[0], strings.NewReader(c[1]), &out, sock, ""); code != 0 || out.Len() != 0 {
			t.Errorf("%s: code %d, stdout %q", name, code, out.String())
		}
	}
	select {
	case line := <-lines:
		t.Fatalf("server got %q", line)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestParseTTY(t *testing.T) {
	for in, want := range map[string]string{"ttys003\n": "/dev/ttys003", "??": "", "??\n": "", "-": "", "": ""} {
		if got := ParseTTY(in); got != want {
			t.Errorf("ParseTTY(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEmptyTTYOmitted(t *testing.T) {
	var buf bytes.Buffer
	if err := proto.WriteLine(&buf, proto.Request{Kind: proto.Hook, Event: "Stop", Payload: json.RawMessage(`{}`), Term: "x"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `"tty"`) {
		t.Fatalf("line has tty key: %s", buf.String())
	}
	buf.Reset()
	proto.WriteLine(&buf, proto.Request{Kind: proto.Hook, TTY: "/dev/ttys003"})
	if !strings.Contains(buf.String(), `"tty":"/dev/ttys003"`) {
		t.Fatalf("line has no tty: %s", buf.String())
	}
}
