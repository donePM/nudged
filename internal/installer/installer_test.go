package installer

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const bin = "/Applications/My Apps/nudged.app/Contents/MacOS/nudged"

const userSettings = `{
  "model": "opus",
  "env": {"A": "1"},
  "hooks": {
    "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "lint.sh"}]}],
    "Stop": [{"hooks": [{"type": "command", "command": "say done && echo <ok>"}]}]
  },
  "statusLine": {"type": "command", "command": "echo 'hi'", "padding": 1}
}`

func setup(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func load(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func groups(m map[string]any, ev string) []any {
	g, _ := m["hooks"].(map[string]any)[ev].([]any)
	return g
}

func statusCmd(m map[string]any) any {
	sl, _ := m["statusLine"].(map[string]any)
	return sl["command"]
}

func nudgedEntries(m map[string]any, ev string) []map[string]any {
	var out []map[string]any
	for _, g := range groups(m, ev) {
		for _, h := range g.(map[string]any)["hooks"].([]any) {
			if isNudged(h.(map[string]any)["command"].(string), "hook") {
				out = append(out, g.(map[string]any))
			}
		}
	}
	return out
}

func TestInstallAddsEntries(t *testing.T) {
	path := setup(t, userSettings)
	if err := Install(path, bin); err != nil {
		t.Fatal(err)
	}
	m, user := load(t, path), load(t, path+".nudged.bak")
	for _, ev := range Events {
		e := nudgedEntries(m, ev)
		if len(e) != 1 {
			t.Fatalf("%s: %d nudged entries", ev, len(e))
		}
		h := e[0]["hooks"].([]any)[0].(map[string]any)
		if h["command"] != Quote(bin)+" hook "+ev || h["timeout"] != float64(1800) || h["type"] != "command" {
			t.Errorf("%s: entry %v", ev, h)
		}
		if e[0]["matcher"] != nil {
			t.Errorf("%s: matcher %v", ev, e[0]["matcher"])
		}
	}
	for _, k := range []string{"model", "env"} {
		if !reflect.DeepEqual(m[k], user[k]) {
			t.Errorf("key %s changed: %v", k, m[k])
		}
	}
	if g := groups(m, "Stop"); !reflect.DeepEqual(g[0], groups(user, "Stop")[0]) || len(g) != 2 {
		t.Errorf("Stop: user hook changed: %v", g)
	}
	if g := groups(m, "PreToolUse"); !reflect.DeepEqual(g, groups(user, "PreToolUse")) {
		t.Errorf("PreToolUse: got %v", g)
	}
	if data, _ := os.ReadFile(path); !bytes.Contains(data, []byte("<ok>")) {
		t.Error("HTML characters are escaped")
	}
}

func TestOldPreToolUseEntry(t *testing.T) {
	old := `{"matcher": "AskUserQuestion", "hooks": [{"type": "command", "command": "'/usr/local/bin/nudged' hook PreToolUse", "timeout": 1800}]}`
	user := `{"matcher": "Bash", "hooks": [{"type": "command", "command": "lint.sh"}]}`
	for _, tc := range []struct {
		name, groups string
		want         int
	}{{"with user hook", user + "," + old, 1}, {"only nudged", old, 0}} {
		for _, op := range []string{"install", "uninstall"} {
			path := setup(t, `{"hooks": {"PreToolUse": [`+tc.groups+`]}}`)
			var err error
			if op == "install" {
				err = Install(path, bin)
			} else {
				err = Uninstall(path)
			}
			if err != nil {
				t.Fatal(err)
			}
			m := load(t, path)
			hooks, _ := m["hooks"].(map[string]any)
			g, has := hooks["PreToolUse"].([]any)
			if len(g) != tc.want || has != (tc.want > 0) {
				t.Fatalf("%s, %s: PreToolUse %v", tc.name, op, hooks["PreToolUse"])
			}
			if tc.want > 0 && !reflect.DeepEqual(g[0], load(t, path+".nudged.bak")["hooks"].(map[string]any)["PreToolUse"].([]any)[0]) {
				t.Errorf("%s, %s: user hook changed", tc.name, op)
			}
		}
	}
}

func TestStatusline(t *testing.T) {
	path := setup(t, userSettings)
	if err := Install(path, bin); err != nil {
		t.Fatal(err)
	}
	m := load(t, path)
	if got, want := statusCmd(m), `'`+bin+`' statusline --wrap 'echo '\''hi'\'''`; got != want {
		t.Errorf("wrap: got %v, want %v", got, want)
	}
	if m["statusLine"].(map[string]any)["padding"] != float64(1) {
		t.Error("statusLine lost padding")
	}
	if err := Uninstall(path); err != nil {
		t.Fatal(err)
	}
	if got := statusCmd(load(t, path)); got != "echo 'hi'" {
		t.Errorf("restore: got %v", got)
	}

	path = setup(t, `{}`)
	if err := Install(path, bin); err != nil {
		t.Fatal(err)
	}
	if got := statusCmd(load(t, path)); got != Quote(bin)+" statusline" {
		t.Errorf("no statusline: got %v", got)
	}
	if err := Uninstall(path); err != nil {
		t.Fatal(err)
	}
	if _, ok := load(t, path)["statusLine"]; ok {
		t.Error("statusLine not removed")
	}
}

func TestInstallIdempotent(t *testing.T) {
	path := setup(t, userSettings)
	if err := Install(path, bin); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	if err := Install(path, bin); err != nil {
		t.Fatal(err)
	}
	if second, _ := os.ReadFile(path); !bytes.Equal(first, second) {
		t.Errorf("second install changed the file:\n%s\n%s", first, second)
	}

	newBin := "/opt/homebrew/bin/nudged"
	if err := Install(path, newBin); err != nil {
		t.Fatal(err)
	}
	m := load(t, path)
	for _, ev := range Events {
		e := nudgedEntries(m, ev)
		if len(e) != 1 || e[0]["hooks"].([]any)[0].(map[string]any)["command"] != Quote(newBin)+" hook "+ev {
			t.Errorf("%s: entries %v", ev, e)
		}
	}
	if got, want := statusCmd(m), Quote(newBin)+" statusline --wrap "+Quote("echo 'hi'"); got != want {
		t.Errorf("statusline: got %v, want %v", got, want)
	}
}

func TestUninstall(t *testing.T) {
	path := setup(t, userSettings)
	before := load(t, path)
	if err := Install(path, bin); err != nil {
		t.Fatal(err)
	}
	// An entry from an old binary path, unquoted, in a group with a user hook.
	m := load(t, path)
	g := groups(m, "Stop")[0].(map[string]any)
	g["hooks"] = append(g["hooks"].([]any), map[string]any{"type": "command", "command": "/usr/local/bin/nudged hook Stop"})
	data, _ := json.Marshal(m)
	os.WriteFile(path, data, 0o644)

	if err := Uninstall(path); err != nil {
		t.Fatal(err)
	}
	if after := load(t, path); !reflect.DeepEqual(before, after) {
		t.Errorf("uninstall result differs:\n%v\n%v", before, after)
	}
	first, _ := os.ReadFile(path)
	bak, _ := os.ReadFile(path + ".nudged.bak")
	if err := Uninstall(path); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	bak2, _ := os.ReadFile(path + ".nudged.bak")
	if !bytes.Equal(first, second) || !bytes.Equal(bak, bak2) {
		t.Error("second uninstall changed a file")
	}

	path = setup(t, `{"hooks":{}}`)
	Install(path, bin)
	Uninstall(path)
	if _, ok := load(t, path)["hooks"]; ok {
		t.Error("empty hooks key not removed")
	}
}

func TestBackupAndCreate(t *testing.T) {
	path := setup(t, userSettings)
	if err := Install(path, bin); err != nil {
		t.Fatal(err)
	}
	if bak, _ := os.ReadFile(path + ".nudged.bak"); string(bak) != userSettings {
		t.Errorf("backup before install: %s", bak)
	}
	installed, _ := os.ReadFile(path)
	if err := Uninstall(path); err != nil {
		t.Fatal(err)
	}
	if bak, _ := os.ReadFile(path + ".nudged.bak"); !bytes.Equal(bak, installed) {
		t.Errorf("backup before uninstall: %s", bak)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o644 {
		t.Errorf("mode changed: %v", fi.Mode())
	}

	path = filepath.Join(t.TempDir(), ".claude", "settings.json")
	if err := Install(path, bin); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("new file: %v %v", fi, err)
	}
	if _, err := os.Stat(path + ".nudged.bak"); !os.IsNotExist(err) {
		t.Error("backup written for a new file")
	}
}

func TestBadJSONKeepsFile(t *testing.T) {
	bad := `{"statusLine": {"command": "x"`
	path := setup(t, bad)
	if Install(path, bin) == nil || Uninstall(path) == nil {
		t.Error("no error for bad JSON")
	}
	if data, _ := os.ReadFile(path); string(data) != bad {
		t.Error("bad file changed")
	}
	if _, err := os.Stat(path + ".nudged.bak"); !os.IsNotExist(err) {
		t.Error("backup written for bad JSON")
	}
}

func TestWords(t *testing.T) {
	for in, want := range map[string][]string{
		Quote(bin) + " statusline --wrap " + Quote(`a 'b' "c"`): {bin, "statusline", "--wrap", `a 'b' "c"`},
		`"/x y/nudged" hook Stop`:                               {"/x y/nudged", "hook", "Stop"},
		`/x\ y/nudged  hook`:                                    {"/x y/nudged", "hook"},
	} {
		if got, err := words(in); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("words(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := words(`'open`); err == nil {
		t.Error("no error for open quote")
	}
	if isNudged("/bin/nudgedx hook Stop", "hook") || isNudged("nudged statusline", "hook") {
		t.Error("false match")
	}
}
