package app

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rokde/nudged/internal/daemon"
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

type fakeTray struct {
	icon  []byte
	label string
}

func (f *fakeTray) setIcon(b []byte)  { f.icon = b }
func (f *fakeTray) setLabel(l string) { f.label = l }

func checkTray(t *testing.T, srv *daemon.Server, f *fakeTray, paused bool) {
	t.Helper()
	icon, label := runningIcon, "Pause"
	if paused {
		icon, label = pausedIcon, "Resume"
	}
	if srv.Paused() != paused || !bytes.Equal(f.icon, icon) || f.label != label {
		t.Fatalf("paused %v, label %q, want paused %v, label %q", srv.Paused(), f.label, paused, label)
	}
}

func testServer(t *testing.T) *daemon.Server {
	t.Helper()
	srv, _, err := startDaemon(sockPath(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	return srv
}

func TestApplyToggle(t *testing.T) {
	srv := testServer(t)
	f := &fakeTray{}
	applyToggle(srv, f.setIcon, f.setLabel)
	checkTray(t, srv, f, true)
	applyToggle(srv, f.setIcon, f.setLabel)
	checkTray(t, srv, f, false)
}

func TestHotkeyPress(t *testing.T) {
	srv := testServer(t)
	f := &fakeTray{}
	var press func()
	old := registerHotkey
	t.Cleanup(func() { registerHotkey = old })
	registerHotkey = func(onPress func()) error { press = onPress; return nil }
	var log bytes.Buffer
	oldLog := logw
	t.Cleanup(func() { logw = oldLog })
	logw = &log

	setupHotkey(func() { applyToggle(srv, f.setIcon, f.setLabel) })
	if press == nil || log.Len() != 0 {
		t.Fatalf("callback %v, log %q", press != nil, log.String())
	}
	press()
	checkTray(t, srv, f, true)
	press()
	checkTray(t, srv, f, false)
}

func TestHotkeyError(t *testing.T) {
	srv := testServer(t)
	f := &fakeTray{}
	old := registerHotkey
	t.Cleanup(func() { registerHotkey = old })
	registerHotkey = func(func()) error { return errors.New("key taken") }
	var log bytes.Buffer
	oldLog := logw
	t.Cleanup(func() { logw = oldLog })
	logw = &log

	setupHotkey(func() { applyToggle(srv, f.setIcon, f.setLabel) })
	lines := strings.Split(strings.TrimSuffix(log.String(), "\n"), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "pause hotkey") {
		t.Fatalf("log %q", log.String())
	}
	applyToggle(srv, f.setIcon, f.setLabel)
	checkTray(t, srv, f, true)
}
