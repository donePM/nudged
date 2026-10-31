package focus

import (
	"strings"
	"testing"
)

func mustScript(t *testing.T, tty, term string) string {
	t.Helper()
	s, ok := Script(tty, term)
	if !ok {
		t.Fatalf("Script(%q, %q) gave false", tty, term)
	}
	return s
}

func wantAll(t *testing.T, s string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(s, p) {
			t.Errorf("script has no %q:\n%s", p, s)
		}
	}
}

func TestTerminal(t *testing.T) {
	s := mustScript(t, "/dev/ttys003", "Apple_Terminal")
	wantAll(t, s, `application "Terminal"`, `"/dev/ttys003"`, "activate", "set index of w to 1")
}

func TestITerm(t *testing.T) {
	s := mustScript(t, "/dev/ttys003", "iTerm.app")
	wantAll(t, s, `application "iTerm"`, `"/dev/ttys003"`, "select", "activate")
}

func TestGhostty(t *testing.T) {
	for _, tty := range []string{"/dev/ttys003", "", `/dev/x" & do shell script "x`} {
		s := mustScript(t, tty, "ghostty")
		wantAll(t, s, `application "Ghostty"`, "activate")
		if tty != "" && strings.Contains(s, tty) {
			t.Errorf("script has the TTY %q", tty)
		}
	}
}

func TestNoScript(t *testing.T) {
	cases := [][2]string{
		{"/dev/ttys003", ""},
		{"/dev/ttys003", "unknown"},
		{"/dev/ttys003", "vscode"},
		{"", "Apple_Terminal"},
		{"", "iTerm.app"},
		{`/dev/ttys003" & do shell script "x`, "Apple_Terminal"},
		{`/dev/ttys003" & do shell script "x`, "iTerm.app"},
		{"/dev/", "Apple_Terminal"},
		{"/dev/../x", "iTerm.app"},
		{"ttys003", "Apple_Terminal"},
		{"/dev/ttys003\n", "Apple_Terminal"},
	}
	for _, c := range cases {
		if s, ok := Script(c[0], c[1]); ok {
			t.Errorf("Script(%q, %q) = %q, true", c[0], c[1], s)
		}
	}
}
