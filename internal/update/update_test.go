package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		cur, latest string
		want        bool
	}{
		{"0.1.4", "0.1.5", true},
		{"0.1.4", "v0.2.0", true},
		{"0.1.9", "0.1.10", true},
		{"0.1.4", "0.1.4", false},
		{"0.2.0", "0.1.9", false},
		{"1.0.0", "0.9.9", false},
		{"dev", "9.9.9", false},
		{"0.1.4", "x", false},
		{"0.1.4", "0.1.5-rc1", false},
	} {
		if got := Newer(c.cur, c.latest); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.cur, c.latest, got)
		}
	}
}

func TestLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			if r.Header.Get("Accept") != "application/vnd.github+json" {
				t.Errorf("accept %q", r.Header.Get("Accept"))
			}
			w.Write([]byte(`{"tag_name":"v0.2.0","html_url":"https://example.com/r","draft":false}`))
		case "/bad-tag":
			w.Write([]byte(`{"tag_name":"nightly"}`))
		case "/bad-json":
			w.Write([]byte(`{`))
		default:
			http.Error(w, "nope", http.StatusForbidden)
		}
	}))
	defer srv.Close()
	r, err := Latest(context.Background(), srv.Client(), srv.URL+"/ok")
	if err != nil || r != (Release{Version: "0.2.0", URL: "https://example.com/r"}) {
		t.Fatalf("ok: %+v %v", r, err)
	}
	for _, p := range []string{"/bad-tag", "/bad-json", "/limit"} {
		if _, err := Latest(context.Background(), srv.Client(), srv.URL+p); err == nil {
			t.Errorf("%s: no error", p)
		}
	}
}

func TestBrewAndInstall(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "log")
	fake := filepath.Join(dir, "brew")
	script := "#!/bin/sh\necho \"$@ $HOMEBREW_NO_AUTO_UPDATE\" >> " + log + "\n[ \"$1\" = upgrade ] && [ -n \"$FAIL\" ] && { echo boom; exit 1; }\nexit 0\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	old := brewPaths
	t.Cleanup(func() { brewPaths = old })
	brewPaths = []string{filepath.Join(dir, "missing"), fake}
	brew, err := Brew()
	if err != nil || brew != fake {
		t.Fatalf("Brew: %q %v", brew, err)
	}
	if err := Install(context.Background(), brew); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(log)
	if got := string(b); got != "update --quiet 1\nupgrade --cask donepm/tap/nudged 1\n" {
		t.Fatalf("calls %q", got)
	}
	t.Setenv("FAIL", "1")
	if err := Install(context.Background(), brew); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("fail: %v", err)
	}
	brewPaths = []string{filepath.Join(dir, "missing")}
	if _, err := Brew(); err != ErrNoBrew {
		t.Fatalf("no brew: %v", err)
	}
}
