package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/rokde/nudged/internal/daemon"
	"github.com/rokde/nudged/internal/proto"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../proto/testdata/hooks/question/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestQuestionView(t *testing.T) {
	if v := viewFor(proto.Question); v != "Question" {
		t.Fatalf("viewFor(question) = %q", v)
	}
	if _, err := os.Stat("../../frontend/src/views/Question.vue"); err != nil {
		t.Fatal(err)
	}
}

func TestQuestionReplyFixture(t *testing.T) {
	got, err := questionReply(readFixture(t, "PermissionRequest.json"), map[string][]string{"Which color do you like?": {"Blue"}})
	if err != nil {
		t.Fatal(err)
	}
	var want bytes.Buffer
	if err := json.Compact(&want, readFixture(t, "PermissionRequest.reply.json")); err != nil {
		t.Fatal(err)
	}
	if string(got) != want.String() {
		t.Fatalf("got %s\nwant %s", got, want.String())
	}
}

const twoQuestions = `{"tool_input":{"questions":[
	{"question":"Colors?","header":"Colors","options":[{"label":"Red","description":"r"},{"label":"Blue","description":"b"}],"multiSelect":true},
	{"question":"Name?","header":"Name","options":[{"label":"A","description":"a"}],"multiSelect":false},
	{"question":"Skipped?","header":"Skip","options":[{"label":"X","description":"x"}],"multiSelect":false}]}}`

func replyAnswers(t *testing.T, out json.RawMessage) map[string]string {
	t.Helper()
	var r struct {
		HookSpecificOutput struct {
			Decision struct {
				UpdatedInput struct {
					Questions []any             `json:"questions"`
					Answers   map[string]string `json:"answers"`
				} `json:"updatedInput"`
			} `json:"decision"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	if n := len(r.HookSpecificOutput.Decision.UpdatedInput.Questions); n != 3 {
		t.Fatalf("questions = %d, want 3", n)
	}
	return r.HookSpecificOutput.Decision.UpdatedInput.Answers
}

func TestQuestionReplyAnswers(t *testing.T) {
	free := `My "own" name, with comma`
	out, err := questionReply(json.RawMessage(twoQuestions), map[string][]string{
		"Colors?": {"Red", "Blue"},
		"Name?":   {free},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"Colors?": "Red, Blue", "Name?": free}
	if got := replyAnswers(t, out); !reflect.DeepEqual(got, want) {
		t.Fatalf("answers = %v, want %v", got, want)
	}
}

func TestQuestionReplyErrors(t *testing.T) {
	for name, payload := range map[string]string{
		"no tool_input": `{}`,
		"no questions":  `{"tool_input":{}}`,
		"null":          `{"tool_input":{"questions":null}}`,
		"empty":         `{"tool_input":{"questions":[]}}`,
		"not json":      `x`,
	} {
		if _, err := questionReply(json.RawMessage(payload), nil); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := questionReply(json.RawMessage(twoQuestions), map[string][]string{"Other?": {"A"}}); err == nil {
		t.Error("unknown question: no error")
	}
}

func TestAnswerQuestion(t *testing.T) {
	pending := []proto.PendingRequest{
		{ID: "1", Kind: proto.Permission, Payload: json.RawMessage(twoQuestions)},
		{ID: "2", Kind: proto.Question, Payload: json.RawMessage(twoQuestions)},
	}
	var calls []string
	answer := func(id string, out json.RawMessage) error {
		calls = append(calls, id)
		return nil
	}
	for name, a := range map[string]questionAnswer{
		"unknown question": {ID: "2", Answers: map[string][]string{"Other?": {"A"}}},
		"unknown id":       {ID: "9", Answers: map[string][]string{"Name?": {"A"}}},
		"permission id":    {ID: "1", Answers: map[string][]string{"Name?": {"A"}}},
	} {
		if err := answerQuestion(pending, a, answer); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if len(calls) != 0 {
		t.Fatalf("answer called on error: %v", calls)
	}
	if err := answerQuestion(pending, questionAnswer{ID: "2", Answers: map[string][]string{"Name?": {"A"}}}, answer); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0] != "2" {
		t.Fatalf("calls = %v", calls)
	}
	failed := errors.New("gone")
	if err := answerQuestion(pending, questionAnswer{ID: "2"}, func(string, json.RawMessage) error { return failed }); err != failed {
		t.Fatalf("err = %v, want %v", err, failed)
	}
}

func TestQuestionTerminal(t *testing.T) {
	reg := daemon.NewRegistry()
	reg.Hook("SessionStart", []byte(`{"session_id":"s1","cwd":"/x/a"}`), "iTerm.app", "/dev/ttys007")
	var q questionTerminal
	// The webview sends only the session id. A tty field has no effect.
	if err := json.Unmarshal([]byte(`{"session_id":"s1","tty":"/dev/evil"}`), &q); err != nil {
		t.Fatal(err)
	}
	type call struct{ tty, term string }
	var calls []call
	focusSession(reg, q.SessionID, func(tty, term string) { calls = append(calls, call{tty, term}) })
	if len(calls) != 1 || calls[0] != (call{"/dev/ttys007", "iTerm.app"}) {
		t.Fatalf("calls = %v", calls)
	}
}
