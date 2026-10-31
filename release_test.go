package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func needTools(t *testing.T, names ...string) {
	t.Helper()
	for _, n := range names {
		if _, err := exec.LookPath(n); err != nil {
			t.Skipf("%s not on PATH", n)
		}
	}
}

func plistValue(t *testing.T, key, file string) string {
	t.Helper()
	out, err := exec.Command("plutil", "-extract", key, "raw", file).CombinedOutput()
	if err != nil {
		t.Fatalf("plutil -extract %s: %v\n%s", key, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestInfoPlist(t *testing.T) {
	needTools(t, "plutil")
	const file = "build/darwin/Info.plist"
	if out, err := exec.Command("plutil", "-lint", file).CombinedOutput(); err != nil {
		t.Fatalf("plutil -lint: %v\n%s", err, out)
	}
	for key, want := range map[string]string{
		"LSUIElement":         "true",
		"CFBundleExecutable":  "nudged",
		"CFBundleIdentifier":  "de.rokde.nudged",
		"CFBundlePackageType": "APPL",
	} {
		if got := plistValue(t, key, file); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestAppBundleScript(t *testing.T) {
	needTools(t, "plutil", "ditto")
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "nudged-bin")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	if out, err := exec.Command("scripts/app-bundle.sh", bin, "1.2.3", tmp).CombinedOutput(); err != nil {
		t.Fatalf("app-bundle.sh: %v\n%s", err, out)
	}
	fi, err := os.Stat(filepath.Join(tmp, "nudged.app/Contents/MacOS/nudged"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Errorf("binary mode %v is not executable", fi.Mode())
	}
	if got := plistValue(t, "CFBundleShortVersionString", filepath.Join(tmp, "nudged.app/Contents/Info.plist")); got != "1.2.3" {
		t.Errorf("CFBundleShortVersionString = %q", got)
	}
	if _, err := os.Stat(filepath.Join(tmp, "nudged_1.2.3_darwin_universal.zip")); err != nil {
		t.Error(err)
	}
}

func fileContains(t *testing.T, file string, want ...string) string {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, w := range want {
		if !strings.Contains(s, w) {
			t.Errorf("%s has no %q", file, w)
		}
	}
	return s
}

func TestGoreleaserConfig(t *testing.T) {
	fileContains(t, ".goreleaser.yaml",
		"version: 2", "CGO_ENABLED=1", "arm64", "amd64", "universal_binaries", "main.version",
		"donePM/homebrew-tap", "owner: donePM", "name: homebrew-tap", "HOMEBREW_TAP_TOKEN", "postflight", "com.apple.quarantine")
}

func TestReleaseWorkflow(t *testing.T) {
	s := fileContains(t, ".github/workflows/release.yml",
		"tags: ['v*']", "macos-latest", "contents: write", "goreleaser/goreleaser-action", "HOMEBREW_TAP_TOKEN")
	guard := strings.Index(s, "hashFiles('frontend/package.json')")
	build := strings.Index(s, "npm --prefix frontend run build")
	if guard < 0 || build < 0 || guard > build {
		t.Errorf("frontend guard at %d, frontend build at %d", guard, build)
	}
}

func TestGoreleaserCheck(t *testing.T) {
	needTools(t, "goreleaser")
	if out, err := exec.Command("goreleaser", "check").CombinedOutput(); err != nil {
		t.Fatalf("goreleaser check: %v\n%s", err, out)
	}
}
