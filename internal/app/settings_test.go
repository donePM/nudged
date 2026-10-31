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
	"github.com/rokde/nudged/internal/loginitem"
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
	oldEmit := emitSettings
	emitSettings = func(*application.App, string, any) {}
	t.Cleanup(func() { newSettingsWindow, settingsWin, emitSettings = old, nil, oldEmit })
	sockPath(t)

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
		"mode":       func(c *config.Config) { c.Mode = "loud" },
		"timeout 0":  func(c *config.Config) { c.TimeoutMinutes = 0 },
		"timeout-1":  func(c *config.Config) { c.TimeoutMinutes = -1 },
		"timeout 31": func(c *config.Config) { c.TimeoutMinutes = 31 },
		"hide":       func(c *config.Config) { c.Events.Waits.HideSeconds = -1 },
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
			err := saveSettings(srv, cfg)
			if err == nil {
				t.Fatal("no error")
			}
			if name == "timeout 31" && !strings.Contains(err.Error(), "30") {
				t.Fatalf("error %q", err)
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

func TestSaveSettingsStartAtLogin(t *testing.T) {
	srv := testServer(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "nudged")
	if err := os.WriteFile(bin, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(bin, link); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(bin)
	if err != nil {
		t.Fatal(err)
	}
	old := executable
	executable = func() (string, error) { return link, nil }
	t.Cleanup(func() { executable = old })
	plist := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", "de.rokde.nudged.plist")

	for _, on := range []bool{true, false} {
		cfg := config.Default()
		cfg.StartAtLogin = on
		if err := saveSettings(srv, cfg); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(plist)
		if on && !strings.Contains(string(data), "<string>"+want+"</string>") {
			t.Errorf("plist %q (%v) has no %s", data, err, want)
		}
		if !on && !os.IsNotExist(err) {
			t.Errorf("plist still there: %v", err)
		}
		got, err := config.Load()
		if err != nil {
			t.Fatal(err)
		}
		if got.StartAtLogin != on {
			t.Errorf("config StartAtLogin %v, want %v", got.StartAtLogin, on)
		}
	}
}

func TestLoadSettingsStartAtLogin(t *testing.T) {
	sockPath(t)
	for _, on := range []bool{true, false} {
		cfg := config.Default()
		cfg.StartAtLogin = !on
		if err := config.Save(cfg); err != nil {
			t.Fatal(err)
		}
		var err error
		if on {
			err = loginitem.Enable("/x/nudged")
		} else {
			err = loginitem.Disable()
		}
		if err != nil {
			t.Fatal(err)
		}
		got, err := loadSettings()
		if err != nil {
			t.Fatal(err)
		}
		if got.StartAtLogin != on || got.StartAtLogin != loginitem.Enabled() {
			t.Errorf("StartAtLogin %v, want %v", got.StartAtLogin, on)
		}
	}
}

func TestSaveSettingsMaxTimeout(t *testing.T) {
	srv := testServer(t)
	cfg := config.Default()
	cfg.TimeoutMinutes = 30
	srv.SetHoldTimeout(time.Minute)
	if err := saveSettings(srv, cfg); err != nil {
		t.Fatal(err)
	}
	if srv.HoldTimeout != 30*time.Minute {
		t.Fatalf("hold timeout %v", srv.HoldTimeout)
	}
}

func TestOpenSettingsReloadsConfig(t *testing.T) {
	sockPath(t)
	oldNew, oldEmit := newSettingsWindow, emitSettings
	newSettingsWindow = func(*application.App, *daemon.Server) settingsWindow { return &fakeWindow{} }
	var got []int
	emitSettings = func(_ *application.App, name string, data any) {
		if name == "settings:config" {
			got = append(got, data.(config.Config).TimeoutMinutes)
		}
	}
	t.Cleanup(func() { newSettingsWindow, settingsWin, emitSettings = oldNew, nil, oldEmit })

	cfg := config.Default()
	cfg.TimeoutMinutes = 5
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	openSettings(nil, nil)
	cfg.TimeoutMinutes = 9
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	openSettings(nil, nil)
	if !slices.Equal(got, []int{5, 9}) {
		t.Fatalf("emitted %v, want [5 9]", got)
	}
}

func TestSettingsViewSource(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "views", "Settings.vue"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"`Test ${r.label} sound`", `max="30"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("Settings.vue lacks %s", want)
		}
	}
	if strings.Contains(string(b), `max="1440"`) {
		t.Error(`Settings.vue still has max="1440"`)
	}
}
