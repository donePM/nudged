package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallHooksCommand(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(t.TempDir(), "nudged")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	for _, sub := range []string{"install-hooks", "uninstall-hooks"} {
		cmd := exec.Command(bin, sub)
		cmd.Env = append(os.Environ(), "HOME="+home)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", sub, err, out)
		}
		data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
		if err != nil {
			t.Fatal(err)
		}
		if has := strings.Contains(string(data), " hook Stop"); has != (sub == "install-hooks") {
			t.Errorf("after %s: %s", sub, data)
		}
	}
}
