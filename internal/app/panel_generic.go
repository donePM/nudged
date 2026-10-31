package app

import (
	"encoding/json"
	"path/filepath"

	"github.com/donepm/nudged/internal/proto"
)

// genericData is what the Generic view shows in notify mode.
type genericData struct {
	Kind     string `json:"kind"`
	Tool     string `json:"tool"`
	Question string `json:"question"`
	Project  string `json:"project"`
}

// genericPanel makes a short summary of req. A bad payload gives empty fields.
func genericPanel(req proto.PendingRequest) genericData {
	var p struct {
		Cwd       string `json:"cwd"`
		ToolName  string `json:"tool_name"`
		ToolInput struct {
			Questions []struct {
				Question string `json:"question"`
			} `json:"questions"`
		} `json:"tool_input"`
	}
	_ = json.Unmarshal(req.Payload, &p)
	d := genericData{Kind: req.Kind}
	if p.Cwd != "" {
		d.Project = filepath.Base(p.Cwd)
	}
	switch req.Kind {
	case proto.Permission:
		d.Tool = p.ToolName
	case proto.Question:
		if len(p.ToolInput.Questions) > 0 {
			d.Question = p.ToolInput.Questions[0].Question
		}
	}
	return d
}
