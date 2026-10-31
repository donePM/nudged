// Package proto defines the socket path and the line-based JSON messages between nudged clients and the daemon.
package proto

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"
)

const SocketMode = 0o600

const (
	Permission = "permission"
	Question   = "question"
	Done       = "done"
	Waits      = "waits"
)

// Hook is the Request.Kind of a message from `nudged hook`.
const Hook = "hook"

// Statusline is the Request.Kind of a message from `nudged statusline`. Its Payload is one StatuslineInfo.
const Statusline = "statusline"

// Status is the Request.Kind of a session query. The Reply.Output is a JSON array of Session.
const Status = "status"

// Pending is the Request.Kind of a queue query. The Reply.Output is a JSON array of PendingRequest, oldest first.
const Pending = "pending"

// Answer is the Request.Kind that answers the held request Request.ID. The daemon sends Request.Payload verbatim as the hook output.
const Answer = "answer"

// PendingRequest is one hook that the daemon holds open (Permission or Question), or a notice (Done or Waits) that does not hold the hook.
type PendingRequest struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	SessionID string          `json:"session_id"`
	Event     string          `json:"event"`
	Payload   json.RawMessage `json:"payload"`
	Term      string          `json:"term"`
	TTY       string          `json:"tty"`
	Started   time.Time       `json:"started"`
}

type StatuslineInfo struct {
	SessionID  string `json:"session_id"`
	Cwd        string `json:"cwd,omitempty"`
	Model      string `json:"model,omitempty"`
	Effort     string `json:"effort,omitempty"`
	ContextPct int    `json:"context_pct"`
}

type Session struct {
	ID           string    `json:"session_id"`
	Project      string    `json:"project"`
	Path         string    `json:"path"`
	Status       string    `json:"status"`
	Term         string    `json:"term"`
	TTY          string    `json:"tty"`
	Model        string    `json:"model"`
	Effort       string    `json:"effort"`
	ContextPct   int       `json:"context_pct"`
	LastActivity time.Time `json:"last_activity"`
}

type Request struct {
	Kind    string          `json:"kind"`
	ID      string          `json:"id,omitempty"`
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload,omitempty"`
	TTY     string          `json:"tty,omitempty"`
	Term    string          `json:"term,omitempty"`
}

type Reply struct {
	OK     bool            `json:"ok"`
	Output json.RawMessage `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func SocketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "nudged", "nudged.sock"), nil
}

// WriteLine writes v as one JSON line. json.Encoder adds the newline.
func WriteLine(w io.Writer, v any) error {
	return json.NewEncoder(w).Encode(v)
}

func ReadLine(r *bufio.Reader, v any) error {
	line, err := r.ReadBytes('\n')
	if err != nil && (err != io.EOF || len(line) == 0) {
		return err
	}
	return json.Unmarshal(line, v)
}
