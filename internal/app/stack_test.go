package app

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/donepm/nudged/internal/config"
	"github.com/donepm/nudged/internal/proto"
)

func TestTop(t *testing.T) {
	list := []proto.PendingRequest{
		{ID: "1", Kind: proto.Question},
		{ID: "2", Kind: proto.Permission},
		{ID: "3", Kind: proto.Question},
		{ID: "4", Kind: proto.Permission},
	}
	cfg := config.Default()
	if req, others, ok := top(list, cfg); !ok || req.ID != "1" || others != 3 {
		t.Fatalf("all shown: %+v %d %v", req, others, ok)
	}
	cfg.Events.Question.Show = false
	if req, others, ok := top(list, cfg); !ok || req.ID != "2" || others != 1 {
		t.Fatalf("questions hidden: %+v %d %v", req, others, ok)
	}
	cfg.Events.Permission.Show = false
	if _, _, ok := top(list, cfg); ok {
		t.Fatal("all hidden: ok")
	}
	if _, _, ok := top(nil, config.Default()); ok {
		t.Fatal("empty list: ok")
	}
}

func TestViewFor(t *testing.T) {
	if v := viewFor("test-kind"); v != "Generic" {
		t.Fatalf("no entry: %q", v)
	}
	registerPanel("test-kind", "Test")
	t.Cleanup(func() { delete(panelViews, "test-kind") })
	if v := viewFor("test-kind"); v != "Test" {
		t.Fatalf("registered: %q", v)
	}
}

func TestDecision(t *testing.T) {
	allow, err := os.ReadFile("../proto/testdata/hooks/PermissionRequest.reply.json")
	if err != nil {
		t.Fatal(err)
	}
	// deny is the tested reply in docs/spike.md.
	for behavior, want := range map[string]string{
		"allow": string(allow),
		"deny":  `{"hookSpecificOutput": {"hookEventName": "PermissionRequest", "decision": {"behavior": "deny", "message": "Denied in nudged panel"}}}`,
	} {
		got, err := decision(behavior)
		if err != nil {
			t.Fatal(err)
		}
		var w bytes.Buffer
		if err := json.Compact(&w, []byte(want)); err != nil {
			t.Fatal(err)
		}
		if string(got) != w.String() {
			t.Fatalf("%s: %s, want %s", behavior, got, w.String())
		}
	}
	if _, err := decision("x"); err == nil {
		t.Fatal("x: no error")
	}
}

func TestPanelView(t *testing.T) {
	for _, kind := range []string{proto.Permission, proto.Question} {
		if v := panelView(&panelShow{Request: proto.PendingRequest{Kind: kind}, Mode: "notify"}); v != "Generic" {
			t.Errorf("%s notify: %q", kind, v)
		}
		if v := panelView(&panelShow{Request: proto.PendingRequest{Kind: kind}, Mode: "answer"}); v != viewFor(kind) || v == "Generic" {
			t.Errorf("%s answer: %q", kind, v)
		}
	}
}

func TestPanelViewNotice(t *testing.T) {
	for _, kind := range []string{proto.Done, proto.Waits} {
		for _, mode := range []string{"answer", "notify"} {
			if v := panelView(&panelShow{Request: proto.PendingRequest{Kind: kind}, Mode: mode}); v != viewFor(kind) {
				t.Errorf("%s %s: %q", kind, mode, v)
			}
		}
	}
	if v := panelView(&panelShow{Request: proto.PendingRequest{Kind: proto.Permission}, Mode: "notify"}); v != "Generic" {
		t.Errorf("permission notify: %q", v)
	}
}

func TestFocusTarget(t *testing.T) {
	req := proto.PendingRequest{ID: "1", TTY: "/dev/ttys003", Term: "iTerm.app"}
	for _, kind := range []string{proto.Done, proto.Waits} {
		for _, mode := range []string{"answer", "notify"} {
			req.Kind = kind
			if tty, term, ok := focusTarget(&panelShow{Request: req, Mode: mode}, "1"); !ok || tty != req.TTY || term != req.Term {
				t.Errorf("%s %s: %q %q %v", kind, mode, tty, term, ok)
			}
			if _, _, ok := focusTarget(&panelShow{Request: req, Mode: mode}, "2"); ok {
				t.Errorf("%s %s other id: ok", kind, mode)
			}
		}
	}
	req.Kind = proto.Permission
	if _, _, ok := focusTarget(&panelShow{Request: req, Mode: "answer"}, "1"); ok {
		t.Error("permission answer: ok")
	}
}
