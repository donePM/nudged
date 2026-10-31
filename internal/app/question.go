package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/donepm/nudged/internal/daemon"
	"github.com/donepm/nudged/internal/focus"
	"github.com/donepm/nudged/internal/proto"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type questionAnswer struct {
	ID      string              `json:"id"`
	Answers map[string][]string `json:"answers"`
}

type questionTerminal struct {
	SessionID string `json:"session_id"`
}

func init() {
	registerPanel(proto.Question, "Question")
	application.RegisterEvent[questionAnswer]("question:answer")
	application.RegisterEvent[questionTerminal]("question:terminal")
}

// questionReply returns the PermissionRequest allow reply with the questions of payload and the answers (format in docs/spike.md, Q1).
// A question with no values gets no key in answers.
func questionReply(payload json.RawMessage, answers map[string][]string) (json.RawMessage, error) {
	var p struct {
		ToolInput struct {
			Questions json.RawMessage `json:"questions"`
		} `json:"tool_input"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	var qs []struct {
		Question string `json:"question"`
	}
	if err := json.Unmarshal(p.ToolInput.Questions, &qs); err != nil || len(qs) == 0 {
		return nil, errors.New("payload has no tool_input.questions")
	}
	known := map[string]bool{}
	for _, q := range qs {
		known[q.Question] = true
	}
	joined := map[string]string{}
	for k, v := range answers {
		if !known[k] {
			return nil, errors.New("no question " + strconv.Quote(k))
		}
		if len(v) > 0 {
			joined[k] = strings.Join(v, ", ")
		}
	}
	type input struct {
		Questions json.RawMessage   `json:"questions"`
		Answers   map[string]string `json:"answers"`
	}
	type dec struct {
		Behavior     string `json:"behavior"`
		UpdatedInput input  `json:"updatedInput"`
	}
	type out struct {
		HookEventName string `json:"hookEventName"`
		Decision      dec    `json:"decision"`
	}
	return json.Marshal(struct {
		HookSpecificOutput out `json:"hookSpecificOutput"`
	}{out{"PermissionRequest", dec{"allow", input{p.ToolInput.Questions, joined}}}})
}

// answerQuestion takes the questions from the held request a.ID, never from the webview. It calls answer only for a valid reply.
func answerQuestion(pending []proto.PendingRequest, a questionAnswer, answer func(string, json.RawMessage) error) error {
	for _, p := range pending {
		if p.ID == a.ID && p.Kind == proto.Question {
			out, err := questionReply(p.Payload, a.Answers)
			if err != nil {
				return err
			}
			return answer(a.ID, out)
		}
	}
	return errors.New("no held question " + strconv.Quote(a.ID))
}

// startQuestion handles the question:* events of the panel.
func startQuestion(app *application.App, srv *daemon.Server) {
	app.Event.On("question:answer", func(e *application.CustomEvent) {
		a, ok := e.Data.(questionAnswer)
		if e.Sender != panelName || !ok {
			return
		}
		if err := answerQuestion(srv.Pending(), a, srv.Answer); err != nil {
			fmt.Fprintln(logw, "nudged: question answer:", err)
		}
	})
	app.Event.On("question:terminal", func(e *application.CustomEvent) {
		if t, ok := e.Data.(questionTerminal); ok && e.Sender == panelName {
			focusSession(srv.Registry, t.SessionID, focus.Jump)
		}
	})
}
