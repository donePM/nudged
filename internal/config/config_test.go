package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/donepm/nudged/internal/proto"
)

func setHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".config", "nudged", "config.toml"); path != want {
		t.Fatalf("path %q, want %q", path, want)
	}
	return path
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadCreatesDefaults(t *testing.T) {
	path := setHome(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Fatalf("got %+v", cfg)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode %o", fi.Mode().Perm())
	}
	di, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if di.Mode().Perm() != 0o700 {
		t.Fatalf("folder mode %o", di.Mode().Perm())
	}
	var back Config
	if _, err := toml.DecodeFile(path, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, Default()) {
		t.Fatalf("file decodes to %+v", back)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temp file left: %v", entries)
	}
}

func TestDefault(t *testing.T) {
	d := Default()
	if d.Mode != "answer" || d.TimeoutMinutes != 30 || !d.StartAtLogin || !d.CheckUpdates || d.Position != "top-right" {
		t.Fatalf("got %+v", d)
	}
	want := Events{
		Permission: Event{Show: true, Sound: false, HideSeconds: 0},
		Question:   Event{Show: true, Sound: false, HideSeconds: 0},
		Done:       Event{Show: true, Sound: false, HideSeconds: 5},
		Waits:      Event{Show: true, Sound: false, HideSeconds: 10},
	}
	if d.Events != want {
		t.Fatalf("events %+v, want %+v", d.Events, want)
	}
}

func TestEventKeysMatchProto(t *testing.T) {
	path := setHome(t)
	if err := Save(Default()); err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		t.Fatal(err)
	}
	events := raw["events"].(map[string]any)
	for _, k := range []string{proto.Permission, proto.Question, proto.Done, proto.Waits} {
		if _, ok := events[k]; !ok {
			t.Errorf("no [events.%s]", k)
		}
	}
}

func TestHandEditsKept(t *testing.T) {
	path := setHome(t)
	writeFile(t, path, "timeout_minutes = 5\n\n[events.done]\nsound = true\n")
	want := Default()
	want.TimeoutMinutes = 5
	want.Events.Done.Sound = true

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg, err = Load(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("after save got %+v, want %+v", cfg, want)
	}
}

func TestLoadErrorKeepsFile(t *testing.T) {
	for name, content := range map[string]string{
		"broken":   "timeout_minutes = = 5\n[events\n",
		"mode":     "mode = \"loud\"\n",
		"position": "position = \"middle\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := setHome(t)
			writeFile(t, path, content)
			if _, err := Load(); err == nil {
				t.Fatal("no error")
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != content {
				t.Fatalf("file changed to %q", got)
			}
		})
	}
}

func TestLoadClampsTimeout(t *testing.T) {
	path := setHome(t)
	for in, want := range map[int]int{1440: 30, 31: 30, 30: 30, 5: 5} {
		content := fmt.Sprintf("timeout_minutes = %d\n", in)
		writeFile(t, path, content)
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.TimeoutMinutes != want {
			t.Errorf("timeout_minutes = %d loads as %d, want %d", in, cfg.TimeoutMinutes, want)
		}
		if b, _ := os.ReadFile(path); string(b) != content {
			t.Errorf("Load changed the file to %q", b)
		}
	}
}
