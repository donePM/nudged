package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSoundUsage(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "nudged")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	for _, args := range [][]string{{"sound"}, {"sound", "other"}} {
		var stderr strings.Builder
		cmd := exec.Command(bin, args...)
		cmd.Stderr = &stderr
		err := cmd.Run()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 {
			t.Errorf("%v: err %v", args, err)
		}
		if !strings.Contains(stderr.String(), "permission|question|notice") {
			t.Errorf("%v: stderr %q", args, stderr.String())
		}
	}
}
