package proto

import (
	"bufio"
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSocketPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := SocketPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".config", "nudged", "nudged.sock"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if SocketMode != 0o600 {
		t.Fatalf("SocketMode %o", SocketMode)
	}
}

func TestRoundTrip(t *testing.T) {
	req := Request{Kind: Permission, Event: "PermissionRequest", Payload: json.RawMessage(`{"tool":"Bash"}`)}
	rep := Reply{OK: true, Output: json.RawMessage(`{"decision":"allow"}`), Error: "none"}

	var buf bytes.Buffer
	if err := WriteLine(&buf, req); err != nil {
		t.Fatal(err)
	}
	first := buf.String()
	if strings.Count(first, "\n") != 1 || !strings.HasSuffix(first, "\n") {
		t.Fatalf("not one line: %q", first)
	}
	if err := WriteLine(&buf, rep); err != nil {
		t.Fatal(err)
	}
	if second := buf.String()[len(first):]; strings.Count(second, "\n") != 1 || !strings.HasSuffix(second, "\n") {
		t.Fatalf("not one line: %q", second)
	}

	r := bufio.NewReader(&buf)
	var gotReq Request
	var gotRep Reply
	if err := ReadLine(r, &gotReq); err != nil {
		t.Fatal(err)
	}
	if err := ReadLine(r, &gotRep); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotReq, req) {
		t.Fatalf("request %+v, want %+v", gotReq, req)
	}
	if !reflect.DeepEqual(gotRep, rep) {
		t.Fatalf("reply %+v, want %+v", gotRep, rep)
	}
}

func TestEventKinds(t *testing.T) {
	for got, want := range map[string]string{Permission: "permission", Question: "question", Done: "done", Waits: "waits"} {
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}
