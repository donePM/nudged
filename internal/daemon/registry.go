// Package daemon holds the session registry and the socket server of nudged.
package daemon

import (
	"cmp"
	"encoding/json"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/rokde/nudged/internal/proto"
)

var hookStatus = map[string]string{
	"SessionStart":     "idle",
	"UserPromptSubmit": "working",
	"PostToolUse":      "working",
	"Notification":     "waiting",
	"Stop":             "done",
	"SessionEnd":       "",
}

// shortcut: a session with no SessionEnd stays in the registry, upgrade when S18 shows dead sessions.
type Registry struct {
	mu       sync.Mutex
	sessions map[string]*proto.Session
	now      func() time.Time
}

func NewRegistry() *Registry {
	return &Registry{sessions: map[string]*proto.Session{}, now: time.Now}
}

// session returns the session with this id. It creates an idle session when the id is unknown. The caller holds mu.
func (r *Registry) session(id, cwd string) *proto.Session {
	s, ok := r.sessions[id]
	if !ok {
		s = &proto.Session{ID: id, Status: "idle"}
		r.sessions[id] = s
	}
	if cwd != "" {
		s.Path, s.Project = cwd, filepath.Base(cwd)
	}
	s.LastActivity = r.now()
	return s
}

// Hook applies one hook event. It ignores unknown events and payloads with no session_id.
func (r *Registry) Hook(event string, payload []byte, term, tty string) {
	status, ok := hookStatus[event]
	var p struct {
		SessionID string `json:"session_id"`
		Cwd       string `json:"cwd"`
	}
	if !ok || json.Unmarshal(payload, &p) != nil || p.SessionID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if event == "SessionEnd" {
		delete(r.sessions, p.SessionID)
		return
	}
	s := r.session(p.SessionID, p.Cwd)
	s.Status = status
	if term != "" {
		s.Term = term
	}
	if tty != "" {
		s.TTY = tty
	}
}

// Statusline sets model, effort and context % of a session. It does not change the status.
func (r *Registry) Statusline(info proto.StatuslineInfo) {
	if info.SessionID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.session(info.SessionID, info.Cwd)
	s.Model, s.Effort, s.ContextPct = info.Model, info.Effort, info.ContextPct
}

// List returns a copy of all sessions: waiting first, then newest activity, then session id.
func (r *Registry) List() []proto.Session {
	r.mu.Lock()
	list := make([]proto.Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		list = append(list, *s)
	}
	r.mu.Unlock()
	slices.SortFunc(list, func(a, b proto.Session) int {
		if (a.Status == "waiting") != (b.Status == "waiting") {
			if a.Status == "waiting" {
				return -1
			}
			return 1
		}
		if c := b.LastActivity.Compare(a.LastActivity); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return list
}
