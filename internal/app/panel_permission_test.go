package app

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/rokde/nudged/internal/proto"
)

func fixtureRequest(t *testing.T) proto.PendingRequest {
	t.Helper()
	b, err := os.ReadFile("../proto/testdata/hooks/PermissionRequest.json")
	if err != nil {
		t.Fatal(err)
	}
	return proto.PendingRequest{
		ID: "r1", Kind: proto.Permission, SessionID: "00000000-0000-4000-8000-000000000001",
		Event: "PermissionRequest", Payload: b, Term: "iTerm.app", TTY: "/dev/ttys003",
		Started: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC),
	}
}

func TestViewForPermission(t *testing.T) {
	if v := viewFor(proto.Permission); v != "Permission" {
		t.Fatalf("view %q", v)
	}
	if _, err := os.Stat("../../frontend/src/views/Permission.vue"); err != nil {
		t.Fatal(err)
	}
}

func TestPermissionPanel(t *testing.T) {
	req := fixtureRequest(t)
	sessions := []proto.Session{
		{ID: "other", Project: "x", Path: "/x", Model: "m"},
		{ID: req.SessionID, Project: "proj", Path: "/work/proj", Model: "claude-opus-5-5"},
	}
	d := permissionPanel(req, sessions, 7)
	if d.Tool != "Bash" || d.Command != "touch d.txt" || d.Params != "" || d.Description != "Create empty file d.txt" {
		t.Fatalf("tool fields: %+v", d)
	}
	if d.Project != "proj" || d.Path != "/work/proj" || d.Model != "claude-opus-5-5" {
		t.Fatalf("session fields: %+v", d)
	}
	if d.Term != "iTerm.app" || d.TTY != "/dev/ttys003" || d.Session != req.SessionID {
		t.Fatalf("terminal fields: %+v", d)
	}
	if !d.Started.Equal(req.Started) || !d.Deadline.Equal(req.Started.Add(7*time.Minute)) {
		t.Fatalf("times: %v %v", d.Started, d.Deadline)
	}
	if d := permissionPanel(req, nil, 0); !d.Deadline.Equal(req.Started.Add(30 * time.Minute)) {
		t.Fatalf("default timeout: %v", d.Deadline)
	}
}

func TestPermissionPanelParams(t *testing.T) {
	req := fixtureRequest(t)
	req.Payload = json.RawMessage(`{"session_id":"s","cwd":"/Users/dev/app","tool_name":"Write","tool_input":{"file_path":"/a.txt","content":"hi"}}`)
	d := permissionPanel(req, nil, 30)
	want := "{\n  \"file_path\": \"/a.txt\",\n  \"content\": \"hi\"\n}"
	if d.Tool != "Write" || d.Command != "" || d.Params != want {
		t.Fatalf("params: %+v", d)
	}
	if d.Project != "app" || d.Path != "/Users/dev/app" || d.Model != "" {
		t.Fatalf("unknown session: %+v", d)
	}
}

func TestPermissionPanelSuggestions(t *testing.T) {
	req := fixtureRequest(t)
	d := permissionPanel(req, nil, 30)
	want := []string{"Always allow /Users/dev/project (session)", "Always allow: mode acceptEdits (session)"}
	if len(d.Suggestions) != len(want) || d.Suggestions[0] != want[0] || d.Suggestions[1] != want[1] {
		t.Fatalf("labels %q", d.Suggestions)
	}
	req.Payload = json.RawMessage(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`)
	if d := permissionPanel(req, nil, 30); d.Suggestions == nil || len(d.Suggestions) != 0 {
		t.Fatalf("no field: %q", d.Suggestions)
	}
	rule := json.RawMessage(`{"type":"addRules","rules":[{"toolName":"Bash","ruleContent":"npm test:*"}],"behavior":"allow","destination":"localSettings"}`)
	if l := suggestionLabel(rule); l != "Always allow Bash(npm test:*) (localSettings)" {
		t.Fatalf("rule label %q", l)
	}
}

func TestPermissionDecisionAlways(t *testing.T) {
	req := fixtureRequest(t)
	out, err := permissionDecision("always", 1, req.Payload)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		HookSpecificOutput struct {
			HookEventName string `json:"hookEventName"`
			Decision      struct {
				Behavior           string          `json:"behavior"`
				UpdatedPermissions json.RawMessage `json:"updatedPermissions"`
			} `json:"decision"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%s: %v", out, err)
	}
	h := got.HookSpecificOutput
	if h.HookEventName != "PermissionRequest" || h.Decision.Behavior != "allow" ||
		string(h.Decision.UpdatedPermissions) != `[{"type":"setMode","mode":"acceptEdits","destination":"session"}]` {
		t.Fatalf("output %s", out)
	}
	for _, b := range []string{"allow", "deny"} {
		want, _ := decision(b)
		got, err := permissionDecision(b, 0, req.Payload)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s: %s %v", b, got, err)
		}
	}
}

func TestPermissionDecisionErrors(t *testing.T) {
	req := fixtureRequest(t)
	for name, c := range map[string]struct {
		behavior string
		index    int
		payload  string
	}{
		"unknown behavior": {"maybe", 0, string(req.Payload)},
		"index too big":    {"always", 2, string(req.Payload)},
		"negative index":   {"always", -1, string(req.Payload)},
		"no suggestions":   {"always", 0, `{"tool_name":"Bash"}`},
		"bad payload":      {"always", 0, `not json`},
	} {
		if out, err := permissionDecision(c.behavior, c.index, json.RawMessage(c.payload)); err == nil || out != nil {
			t.Fatalf("%s: %s %v", name, out, err)
		}
	}
}
