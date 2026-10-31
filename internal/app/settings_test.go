package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rokde/nudged/internal/config"
	"github.com/rokde/nudged/internal/daemon"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type fakeWindow struct{ shows int }

func (w *fakeWindow) show() { w.shows++ }

func TestOpenSettingsOneWindow(t *testing.T) {
	var made []*fakeWindow
	old := newSettingsWindow
	newSettingsWindow = func(*application.App, *daemon.Server) settingsWindow {
		w := &fakeWindow{}
		made = append(made, w)
		return w
	}
	t.Cleanup(func() { newSettingsWindow, settingsWin = old, nil })

	openSettings(nil, nil)
	openSettings(nil, nil)
	if len(made) != 1 {
		t.Fatalf("%d windows, want 1", len(made))
	}
	if made[0].shows != 2 {
		t.Fatalf("%d shows, want 2", made[0].shows)
	}
	if settingsURL != "/#/Settings" {
		t.Fatalf("url %q", settingsURL)
	}
}

func TestSaveSettings(t *testing.T) {
	srv := testServer(t)
	cfg := config.Default()
	cfg.Mode = "notify"
	cfg.TimeoutMinutes = 7
	cfg.Events.Done = config.Event{Show: false, Sound: true, HideSeconds: 3}
	if err := saveSettings(srv, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != cfg {
		t.Fatalf("got %+v, want %+v", got, cfg)
	}
	if srv.HoldTimeout != 7*time.Minute {
		t.Fatalf("hold timeout %v", srv.HoldTimeout)
	}
}

func TestDecodeConfigKeepsFields(t *testing.T) {
	sockPath(t)
	cfg, err := decodeConfig(map[string]any{"Mode": "notify", "TimeoutMinutes": 9.0})
	if err != nil {
		t.Fatal(err)
	}
	want := config.Default()
	want.Mode, want.TimeoutMinutes = "notify", 9
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestSaveSettingsRejects(t *testing.T) {
	cases := map[string]func(*config.Config){
		"mode":      func(c *config.Config) { c.Mode = "loud" },
		"timeout 0": func(c *config.Config) { c.TimeoutMinutes = 0 },
		"timeout-1": func(c *config.Config) { c.TimeoutMinutes = -1 },
		"hide":      func(c *config.Config) { c.Events.Waits.HideSeconds = -1 },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			srv := testServer(t)
			if err := config.Save(config.Default()); err != nil {
				t.Fatal(err)
			}
			path, _ := config.Path()
			before, _ := os.ReadFile(path)
			cfg := config.Default()
			change(&cfg)
			if err := saveSettings(srv, cfg); err == nil {
				t.Fatal("no error")
			}
			after, _ := os.ReadFile(path)
			if string(after) != string(before) {
				t.Fatalf("file changed to %s", after)
			}
			if srv.HoldTimeout != 30*time.Minute {
				t.Fatalf("hold timeout changed to %v", srv.HoldTimeout)
			}
		})
	}
}

func TestTestSound(t *testing.T) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var got []string
	old := play
	play = func(name string) error {
		mu.Lock()
		got = append(got, name)
		mu.Unlock()
		wg.Done()
		return nil
	}
	t.Cleanup(func() { play = old })

	for _, key := range []string{"permission", "question", "done", "waits"} {
		wg.Add(1)
		if err := testSound(key); err != nil {
			t.Fatal(err)
		}
		wg.Wait() // one at a time keeps the order
	}
	if want := []string{"permission", "question", "notice", "notice"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if err := testSound("bogus"); err == nil {
		t.Fatal("no error for unknown key")
	}
	time.Sleep(10 * time.Millisecond)
	if len(got) != 4 {
		t.Fatalf("play called for unknown key: %v", got)
	}
}

func TestRunHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".claude", "settings.json")
	bin := filepath.Join(t.TempDir(), "nudged")
	if err := os.WriteFile(bin, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	old := executable
	executable = func() (string, error) { return bin, nil }
	t.Cleanup(func() { executable = old })
	for _, action := range []string{"install", "remove"} {
		if _, err := runHooks(action); err != nil {
			t.Fatalf("%s: %v", action, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if has := strings.Contains(string(data), " hook Stop"); has != (action == "install") {
			t.Errorf("after %s: %s", action, data)
		}
	}
	if _, err := runHooks("format-disk"); err == nil {
		t.Fatal("no error for unknown action")
	}
}
