package proto

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const hooksDir = "testdata/hooks"

// fakeSessionID is the only session id that the scrubbed fixtures may contain.
const fakeSessionID = "00000000-0000-4000-8000-000000000001"

var hookEvents = []string{"PermissionRequest", "PreToolUse", "Stop", "Notification", "SessionStart", "SessionEnd", "UserPromptSubmit", "PostToolUse"}

// spikeSections returns the text of each "## Q" section of docs/spike.md, in file order.
func spikeSections(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("../../docs/spike.md")
	if err != nil {
		t.Fatal(err)
	}
	var sections []string
	in := false
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "## ") {
			in = strings.HasPrefix(line, "## Q")
			if in {
				sections = append(sections, "")
			}
		}
		if in {
			sections[len(sections)-1] += line + "\n"
		}
	}
	return sections
}

func readJSON(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(hooksDir, name))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return m
}

func TestSpikeQuestions(t *testing.T) {
	sections := spikeSections(t)
	if len(sections) != 9 {
		t.Fatalf("got %d question sections, want 9", len(sections))
	}
	for i, s := range sections {
		head, _, _ := strings.Cut(s, "\n")
		rest, ok := strings.CutPrefix(head, fmt.Sprintf("## Q%d", i+1))
		if !ok || (rest != "" && rest[0] >= '0' && rest[0] <= '9') {
			t.Errorf("section %d heading %q, want ## Q%d", i+1, head, i+1)
		}
		for _, key := range []string{"Answer:", "Evidence:", "Decision:"} {
			n := 0
			for _, line := range strings.Split(s, "\n") {
				if v, ok := strings.CutPrefix(line, key); ok {
					n++
					if strings.TrimSpace(v) == "" {
						t.Errorf("Q%d: empty %s line", i+1, key)
					}
				}
			}
			if n != 1 {
				t.Errorf("Q%d: %d lines start with %s, want 1", i+1, n, key)
			}
		}
	}
}

func TestHookSamples(t *testing.T) {
	for _, ev := range hookEvents {
		if _, err := os.Stat(filepath.Join(hooksDir, ev+".json")); err != nil {
			t.Errorf("missing sample: %v", err)
		}
	}
	readJSON(t, "statusline.json")

	// Every stdin sample, also in subfolders, carries its own event name.
	err := filepath.WalkDir(hooksDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(path, ".reply.json") || d.Name() == "statusline.json" {
			return err
		}
		m := readJSON(t, strings.TrimPrefix(path, hooksDir+"/"))
		if want := strings.TrimSuffix(d.Name(), ".json"); m["hook_event_name"] != want {
			t.Errorf("%s: hook_event_name %v, want %s", path, m["hook_event_name"], want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestToolSamples(t *testing.T) {
	pre := readJSON(t, "PreToolUse.json")
	if pre["tool_name"] != "AskUserQuestion" {
		t.Errorf("PreToolUse tool_name %v", pre["tool_name"])
	}
	in, _ := pre["tool_input"].(map[string]any)
	if qs, _ := in["questions"].([]any); len(qs) == 0 {
		t.Errorf("PreToolUse tool_input has no questions: %v", pre["tool_input"])
	}

	perm := readJSON(t, "PermissionRequest.json")
	if _, ok := perm["tool_name"].(string); !ok {
		t.Errorf("PermissionRequest has no tool_name")
	}
	if _, ok := perm["tool_input"].(map[string]any); !ok {
		t.Errorf("PermissionRequest has no tool_input")
	}
}

func hookOutput(t *testing.T, name string) map[string]any {
	t.Helper()
	out, _ := readJSON(t, name)["hookSpecificOutput"].(map[string]any)
	if out == nil {
		t.Fatalf("%s: no hookSpecificOutput", name)
	}
	return out
}

func TestReplies(t *testing.T) {
	perm := hookOutput(t, "PermissionRequest.reply.json")
	if perm["hookEventName"] != "PermissionRequest" {
		t.Errorf("PermissionRequest reply hookEventName %v", perm["hookEventName"])
	}
	if _, ok := perm["decision"].(map[string]any); !ok {
		t.Errorf("PermissionRequest reply has no decision")
	}

	q1 := spikeSections(t)[0]
	_, after, _ := strings.Cut(q1, "\nAnswer:")
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(after)), "yes") {
		return
	}
	if pre := hookOutput(t, "PreToolUse.reply.json"); pre["hookEventName"] != "PreToolUse" {
		t.Errorf("PreToolUse reply hookEventName %v", pre["hookEventName"])
	}
}

func TestFixturesScrubbed(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(hooksDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), home) {
			t.Errorf("%s contains the home path %s", path, home)
		}
		var m map[string]any
		if json.Unmarshal(b, &m) == nil {
			if id, ok := m["session_id"]; ok && id != fakeSessionID {
				t.Errorf("%s: session_id %v, want the fake id", path, id)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
