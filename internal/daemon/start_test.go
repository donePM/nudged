package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeCfg(t *testing.T, content string) {
	t.Helper()
	home, err := os.MkdirTemp("", "nd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "nudged")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStartTimeout(t *testing.T) {
	writeCfg(t, "timeout_minutes = 7\n")
	srv, cfg, err := Start(sockPath(t), "p:")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	if srv.HoldTimeout != 7*time.Minute || cfg.TimeoutMinutes != 7 {
		t.Fatalf("HoldTimeout = %v", srv.HoldTimeout)
	}
}

func TestStartZeroTimeoutKeepsDefault(t *testing.T) {
	writeCfg(t, "timeout_minutes = 0\n")
	srv, _, err := Start(sockPath(t), "p:")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	if srv.HoldTimeout != 0 {
		t.Fatalf("HoldTimeout = %v", srv.HoldTimeout)
	}
}

func TestStartConfigErrorPrintsPrefix(t *testing.T) {
	writeCfg(t, "timeout_minutes = = 5\n")
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	srv, _, err := Start(sockPath(t), "pfx:")
	os.Stderr = old
	w.Close()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	if !strings.HasPrefix(string(buf[:n]), "pfx: ") {
		t.Fatalf("stderr = %q", buf[:n])
	}
}

func TestStartListenFails(t *testing.T) {
	writeCfg(t, "")
	path := sockPath(t)
	first := start(t, path, NewRegistry())
	_ = first
	srv, _, err := Start(path, "p:")
	if err == nil || srv != nil {
		t.Fatalf("srv=%v err=%v", srv, err)
	}
}
