package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUsageCommand(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	line := `{"type":"assistant","timestamp":"` + ts + `","requestId":"r","message":{"id":"m","model":"claude-sonnet-4-5","content":"secret","usage":{"input_tokens":1000000,"output_tokens":20,"cache_creation_input_tokens":30,"cache_read_input_tokens":40}}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "nudged")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, "usage")
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("exit: %v", err)
	}
	for _, want := range []string{"input        1000000\n", "output       20\n", "cache write  30\n", "cache read   40\n", "cost         ≈ $3.00\n"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output %q has no %q", out, want)
		}
	}
	if strings.Contains(string(out), "secret") {
		t.Error("output shows log content")
	}
}
