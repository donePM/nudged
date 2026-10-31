package app

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/rokde/nudged/internal/proto"
)

func sockPath(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dir, err := os.MkdirTemp("", "nd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "s.sock")
}

func send(t *testing.T, path string, req proto.Request) proto.Reply {
	t.Helper()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if err := proto.WriteLine(conn, req); err != nil {
		t.Fatal(err)
	}
	var rep proto.Reply
	if err := proto.ReadLine(bufio.NewReader(conn), &rep); err != nil {
		t.Fatal(err)
	}
	return rep
}

func TestStartDaemon(t *testing.T) {
	path := sockPath(t)
	srv, _, err := startDaemon(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}
	if rep := send(t, path, proto.Request{Kind: proto.Status}); !rep.OK || string(rep.Output) != "[]" {
		t.Fatalf("status reply %+v", rep)
	}
	if _, _, err := startDaemon(path); err == nil {
		t.Fatal("second start: no error")
	}
	if code := run(path); code != 1 {
		t.Fatalf("run exit code %d", code)
	}
}

func TestMenuItems(t *testing.T) {
	want := []menuItem{
		{"Pause", "Option+CmdOrCtrl+P"},
		{"Settings…", "CmdOrCtrl+,"},
		{"Quit", "CmdOrCtrl+Q"},
	}
	if !slices.Equal(menuItems, want) {
		t.Fatalf("menu %+v", menuItems)
	}
}

func TestToggle(t *testing.T) {
	if bytes.Equal(runningIcon, pausedIcon) || len(runningIcon) == 0 || len(pausedIcon) == 0 {
		t.Fatal("icons are empty or equal")
	}
	path := sockPath(t)
	srv, _, err := startDaemon(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })

	icon, label := toggle(srv)
	if !srv.Paused() || !bytes.Equal(icon, pausedIcon) || label != "Resume" {
		t.Fatalf("paused %v, label %q", srv.Paused(), label)
	}
	rep := send(t, path, proto.Request{Kind: proto.Hook, Event: "PermissionRequest", Payload: json.RawMessage(`{"session_id":"a","tool_name":"Bash"}`)})
	if !rep.OK || rep.Output != nil {
		t.Fatalf("hook reply %+v", rep)
	}

	icon, label = toggle(srv)
	if srv.Paused() || !bytes.Equal(icon, runningIcon) || label != "Pause" {
		t.Fatalf("paused %v, label %q", srv.Paused(), label)
	}
}

func TestQuit(t *testing.T) {
	path := sockPath(t)
	srv, errc, err := startDaemon(path)
	if err != nil {
		t.Fatal(err)
	}
	quitted := false
	quit(srv, func() { quitted = true })
	if !quitted {
		t.Fatal("app did not quit")
	}
	if err := <-errc; err != nil {
		t.Fatalf("serve: %v", err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("socket file: %v", err)
	}
}
