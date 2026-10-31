package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/rokde/nudged/internal/proto"
)

func init() { registerPanel(proto.Permission, "Permission") }

// permissionData is what the Permission view shows.
type permissionData struct {
	Tool        string    `json:"tool"`
	Command     string    `json:"command"`
	Params      string    `json:"params"`
	Description string    `json:"description"`
	Project     string    `json:"project"`
	Path        string    `json:"path"`
	Term        string    `json:"term"`
	TTY         string    `json:"tty"`
	Session     string    `json:"session"`
	Model       string    `json:"model"`
	Started     time.Time `json:"started"`
	Deadline    time.Time `json:"deadline"`
	Suggestions []string  `json:"suggestions"`
}

type permissionPayload struct {
	Cwd         string            `json:"cwd"`
	ToolName    string            `json:"tool_name"`
	ToolInput   json.RawMessage   `json:"tool_input"`
	Suggestions []json.RawMessage `json:"permission_suggestions"`
}

// permissionPanel makes the panel data of req. Project, path and model come from the session in sessions, else from cwd. timeoutMinutes <= 0 means 30, as in the daemon.
func permissionPanel(req proto.PendingRequest, sessions []proto.Session, timeoutMinutes int) permissionData {
	var p permissionPayload
	var in struct{ Command, Description string }
	_ = json.Unmarshal(req.Payload, &p)
	_ = json.Unmarshal(p.ToolInput, &in)
	if timeoutMinutes <= 0 {
		timeoutMinutes = 30
	}
	d := permissionData{
		Tool: p.ToolName, Command: in.Command, Description: in.Description,
		Path: p.Cwd, Term: req.Term, TTY: req.TTY, Session: req.SessionID,
		Started: req.Started, Deadline: req.Started.Add(time.Duration(timeoutMinutes) * time.Minute),
		Suggestions: []string{},
	}
	if p.Cwd != "" {
		d.Project = filepath.Base(p.Cwd)
	}
	if d.Command == "" && len(p.ToolInput) > 0 {
		var b bytes.Buffer
		if json.Indent(&b, p.ToolInput, "", "  ") == nil {
			d.Params = b.String()
		}
	}
	for _, s := range sessions {
		if s.ID == req.SessionID {
			d.Project, d.Path, d.Model = s.Project, s.Path, s.Model
			break
		}
	}
	for _, s := range p.Suggestions {
		d.Suggestions = append(d.Suggestions, suggestionLabel(s))
	}
	return d
}

// suggestionLabel describes one permission_suggestions entry for a button.
func suggestionLabel(raw json.RawMessage) string {
	var s struct {
		Type        string   `json:"type"`
		Mode        string   `json:"mode"`
		Behavior    string   `json:"behavior"`
		Destination string   `json:"destination"`
		Directories []string `json:"directories"`
		Rules       []struct {
			ToolName    string `json:"toolName"`
			RuleContent string `json:"ruleContent"`
		} `json:"rules"`
	}
	_ = json.Unmarshal(raw, &s)
	var label string
	switch s.Type {
	case "addRules":
		var rules []string
		for _, r := range s.Rules {
			if r.RuleContent != "" {
				rules = append(rules, r.ToolName+"("+r.RuleContent+")")
			} else {
				rules = append(rules, r.ToolName)
			}
		}
		b := s.Behavior
		if b == "" {
			b = "allow"
		}
		label = "Always " + b + " " + strings.Join(rules, ", ")
	case "setMode":
		label = "Always allow: mode " + s.Mode
	case "addDirectories":
		label = "Always allow " + strings.Join(s.Directories, ", ")
	default:
		label = "Always allow: " + s.Type
	}
	if s.Destination != "" {
		label += " (" + s.Destination + ")"
	}
	return label
}

// permissionDecision makes the hook output for behavior "allow", "deny" or "always". "always" sends entry index of permission_suggestions from payload, unchanged.
func permissionDecision(behavior string, index int, payload json.RawMessage) (json.RawMessage, error) {
	if behavior != "always" {
		return decision(behavior)
	}
	var p permissionPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	if index < 0 || index >= len(p.Suggestions) {
		return nil, errors.New("no permission suggestion at this index")
	}
	type dec struct {
		Behavior           string            `json:"behavior"`
		UpdatedPermissions []json.RawMessage `json:"updatedPermissions"`
	}
	type hso struct {
		HookEventName string `json:"hookEventName"`
		Decision      dec    `json:"decision"`
	}
	return json.Marshal(struct {
		HookSpecificOutput hso `json:"hookSpecificOutput"`
	}{hso{"PermissionRequest", dec{"allow", []json.RawMessage{p.Suggestions[index]}}}})
}
