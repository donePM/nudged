package statusline

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

	"github.com/rokde/nudged/internal/proto"
)

const fixture = `{"session_id":"abc","cwd":"/tmp/p","model":{"id":"claude-opus","display_name":"Opus"},"effort":"high","context_window":{"used_percentage":41.6}}`

func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "nd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestParse(t *testing.T) {
	got, err := Parse([]byte(fixture))
	want := proto.StatuslineInfo{SessionID: "abc", Cwd: "/tmp/p", Model: "Opus", Effort: "high", ContextPct: 42}
	if err != nil || got != want {
		t.Fatalf("got %+v, %v", got, err)
	}
	got, err = Parse([]byte(`{"session_id":"abc","model":{"display_name":"Opus"}}`))
	if err != nil || got.ContextPct != 0 || got.Effort != "" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestSend(t *testing.T) {
	sock := filepath.Join(shortDir(t), "s.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	lines := make(chan string, 2)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			b, _ := bufio.NewReader(conn).ReadString(0)
			lines <- b
			conn.Close()
		}
	}()
	var out bytes.Buffer
	if code := Run(nil, strings.NewReader(fixture), &out, sock); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	line := <-lines
	if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
		t.Fatalf("not one line: %q", line)
	}
	var req proto.Request
	if err := json.Unmarshal([]byte(line), &req); err != nil || req.Kind != "statusline" {
		t.Fatalf("req %+v, %v", req, err)
	}
	var keys map[string]any
	json.Unmarshal(req.Payload, &keys)
	for _, k := range []string{"session_id", "cwd", "model", "effort", "context_pct"} {
		if _, ok := keys[k]; !ok {
			t.Errorf("payload has no %s: %s", k, req.Payload)
		}
	}
	var info proto.StatuslineInfo
	json.Unmarshal(req.Payload, &info)
	if want := (proto.StatuslineInfo{SessionID: "abc", Cwd: "/tmp/p", Model: "Opus", Effort: "high", ContextPct: 42}); info != want {
		t.Fatalf("info %+v", info)
	}
	select {
	case l := <-lines:
		t.Fatalf("second line %q", l)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestWrap(t *testing.T) {
	sock := filepath.Join(shortDir(t), "none.sock")
	var out bytes.Buffer
	if code := Run([]string{"--wrap", "cat"}, strings.NewReader(fixture), &out, sock); code != 0 || out.String() != fixture {
		t.Fatalf("cat: code %d, stdout %q", code, out.String())
	}
	out.Reset()
	if code := Run([]string{"--wrap", "echo hi; exit 3"}, strings.NewReader(fixture), &out, sock); code != 0 || out.String() != "hi\n" {
		t.Fatalf("exit 3: code %d, stdout %q", code, out.String())
	}
}

func TestDefaultLine(t *testing.T) {
	var out bytes.Buffer
	Run(nil, strings.NewReader(fixture), &out, filepath.Join(shortDir(t), "none.sock"))
	if got := out.String(); got != "Opus · high · 42% ctx\n" {
		t.Fatalf("stdout %q", got)
	}
	out.Reset()
	Run(nil, strings.NewReader(`{"model":{"display_name":"Opus"},"context_window":{"used_percentage":7}}`), &out, "")
	if got := out.String(); got != "Opus · 7% ctx\n" {
		t.Fatalf("stdout %q", got)
	}
}

func TestNoDaemon(t *testing.T) {
	dir := shortDir(t)
	hang := filepath.Join(dir, "hang.sock")
	ln, err := net.Listen("unix", hang)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		var held []net.Conn
		for {
			c, err := ln.Accept()
			if err != nil {
				for _, c := range held {
					c.Close()
				}
				return
			}
			held = append(held, c) // never read, never reply
		}
	}()
	for name, sock := range map[string]string{"missing": filepath.Join(dir, "none.sock"), "hang": hang} {
		for _, args := range [][]string{nil, {"--wrap", "cat"}} {
			var out bytes.Buffer
			start := time.Now()
			code := Run(args, strings.NewReader(fixture), &out, sock)
			if d := time.Since(start); code != 0 || out.Len() == 0 || d >= time.Second {
				t.Errorf("%s %v: code %d, stdout %q, took %v", name, args, code, out.String(), d)
			}
		}
	}
}

func TestBadJSON(t *testing.T) {
	var out bytes.Buffer
	if code := Run([]string{"--wrap", "cat"}, strings.NewReader(`{"model":`), &out, ""); code != 0 || out.String() != `{"model":` {
		t.Fatalf("code %d, stdout %q", code, out.String())
	}
}
