package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var buf bytes.Buffer
	if code := printVersion(&buf); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if got := buf.String(); got != "nudged dev\n" {
		t.Fatalf("got %q", got)
	}
}

func TestVersionLdflags(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "nudged")
	if out, err := exec.Command("go", "build", "-ldflags", "-X main.version=1.2.3", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	out, err := exec.Command(bin, "version").Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "nudged 1.2.3\n" {
		t.Fatalf("got %q", out)
	}
}

func runWithStderr(t *testing.T, args []string) (int, string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	code := run(args)
	os.Stderr = old
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return code, buf.String()
}

func TestUnknownCommand(t *testing.T) {
	code, stderr := runWithStderr(t, []string{"nope"})
	if code != 2 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(stderr, "version") {
		t.Fatalf("usage %q has no version", stderr)
	}
}

func TestNoCommand(t *testing.T) {
	code, stderr := runWithStderr(t, nil)
	if code != 2 || !strings.Contains(stderr, "usage:") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestCIWorkflow(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"runs-on: macos-latest", "go-version-file: go.mod", "go vet ./...", "go test ./..."} {
		if !strings.Contains(string(data), want) {
			t.Errorf("ci.yml has no %q", want)
		}
	}
}

func TestConfigCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	code := run([]string{"config"})
	os.Stdout = old
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	first, _, _ := strings.Cut(out, "\n")
	if want := filepath.Join(home, ".config", "nudged", "config.toml"); first != want {
		t.Fatalf("first line %q, want %q", first, want)
	}
	if !strings.Contains(out, "timeout_minutes = 30") {
		t.Fatalf("output %q", out)
	}
}

func TestConfigCommandError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".config", "nudged", "config.toml")
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, []byte("mode = \"loud\"\n"), 0o600)
	var stdout, stderr bytes.Buffer
	if code := printConfig(&stdout, &stderr); code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "loud") {
		t.Fatalf("code %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}
