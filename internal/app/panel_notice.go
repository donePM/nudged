package app

import (
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/donepm/nudged/internal/proto"
)

func init() {
	registerPanel(proto.Done, "Notice")
	registerPanel(proto.Waits, "Notice")
}

// noticeData is what the Notice view shows.
type noticeData struct {
	Title   string `json:"title"`
	Project string `json:"project"`
	Session string `json:"session"`
	// Duration is the time in whole seconds from the last prompt to the Stop. It is 0 for waits.
	Duration int       `json:"duration"`
	Since    time.Time `json:"since"`
}

// noticePanel makes the panel data of the notice req. The project comes from the session in sessions, else from the payload cwd.
func noticePanel(req proto.PendingRequest, sessions []proto.Session) noticeData {
	var p struct {
		Cwd string `json:"cwd"`
	}
	_ = json.Unmarshal(req.Payload, &p)
	d := noticeData{Title: "Session waits", Session: req.SessionID, Since: req.Started}
	if p.Cwd != "" {
		d.Project = filepath.Base(p.Cwd)
	}
	var prompt time.Time
	for _, s := range sessions {
		if s.ID == req.SessionID {
			d.Project, prompt = s.Project, s.PromptStarted
			break
		}
	}
	if req.Kind == proto.Done {
		d.Title = "Session done"
		if !prompt.IsZero() {
			d.Duration = max(0, int(req.Started.Sub(prompt)/time.Second))
		}
	}
	return d
}
