package daemon

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rokde/nudged/internal/hookclient"
	"github.com/rokde/nudged/internal/proto"
)

// waitSnap reads snapshots until one has the request ids want, in this order.
func waitSnap(t *testing.T, srv *Server, want ...string) {
	t.Helper()
	timeout := time.After(2 * time.Second)
	var last []proto.PendingRequest
	for {
		select {
		case last = <-srv.Changes():
			ids := make([]string, len(last))
			for i, p := range last {
				ids[i] = p.ID
			}
			if slices.Equal(ids, want) {
				return
			}
		case <-timeout:
			t.Fatalf("last snapshot %+v, want ids %v", last, want)
		}
	}
}

func TestChanges(t *testing.T) {
	path := sockPath(t)
	srv := startWith(t, path, 300*time.Millisecond, 0)

	_, ch := holdHook(t, path, "PermissionRequest", perm1)
	waitSnap(t, srv, "1")
	if err := srv.Answer("1", json.RawMessage(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	waitSnap(t, srv)
	reply(t, ch, `{"ok":true,"output":{"a":1}}`)

	// timeout
	holdHook(t, path, "PermissionRequest", perm1)
	waitSnap(t, srv, "2")
	waitSnap(t, srv)

	// client disconnect
	conn, _ := holdHook(t, path, "PermissionRequest", perm1)
	waitSnap(t, srv, "3")
	conn.Close()
	waitSnap(t, srv)

	// next event of the same session
	holdHook(t, path, "PermissionRequest", perm1)
	waitSnap(t, srv, "4")
	hook(t, path, "PostToolUse", `{"session_id":"s1"}`)
	waitSnap(t, srv)

	// pause
	holdHook(t, path, "PermissionRequest", perm1)
	holdHook(t, path, "PermissionRequest", ask2)
	waitSnap(t, srv, "5", "6")
	srv.SetPaused(true)
	waitSnap(t, srv)
}

func TestChangesNoReader(t *testing.T) {
	path := sockPath(t)
	srv := start(t, path, NewRegistry())
	for range 3 {
		_, ch := holdHook(t, path, "PermissionRequest", perm1)
		list := waitPending(t, path, 1)
		if err := srv.Answer(list[0].ID, json.RawMessage(`{}`)); err != nil {
			t.Fatal(err)
		}
		reply(t, ch, `{"ok":true,"output":{}}`)
	}
}

func TestServerAnswer(t *testing.T) {
	path := sockPath(t)
	srv := start(t, path, NewRegistry())
	done := make(chan string, 1)
	go func() {
		var out bytes.Buffer
		hookclient.Run("PermissionRequest", strings.NewReader(perm1), &out, path, "iTerm.app")
		done <- out.String()
	}()
	waitPending(t, path, 1)
	if err := srv.Answer("99", json.RawMessage(`{}`)); err == nil {
		t.Fatal("unknown id accepted")
	}
	payload := `{"hookSpecificOutput":{"hookEventName":"PermissionRequest","decision":{"behavior":"deny"}}}`
	if err := srv.Answer("1", json.RawMessage(payload)); err != nil {
		t.Fatal(err)
	}
	if got := <-done; got != payload+"\n" {
		t.Fatalf("stdout %q", got)
	}
	waitPending(t, path, 0)
}
