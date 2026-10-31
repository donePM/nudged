// Package proto defines the socket path and the line-based JSON messages between nudged clients and the daemon.
package proto

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

const SocketMode = 0o600

const (
	Permission = "permission"
	Question   = "question"
	Done       = "done"
	Waits      = "waits"
)

type Request struct {
	Kind    string          `json:"kind"`
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload,omitempty"`
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
