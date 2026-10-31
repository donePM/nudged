// Package statusline sends the session status line data to the daemon and prints the status line.
package statusline

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/donepm/nudged/internal/proto"
)

// shortcut: 200 ms in total for the socket, make it a config value when users see lost updates.
const sendTimeout = 200 * time.Millisecond

// Run always returns 0 and writes no errors: a failure must never break the status line of Claude Code.
func Run(args []string, stdin io.Reader, stdout io.Writer, socket string) int {
	fs := flag.NewFlagSet("statusline", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	wrap := fs.String("wrap", "", "")
	fs.Parse(args)
	data, _ := io.ReadAll(stdin)
	info, err := Parse(data)
	if err == nil && socket != "" {
		send(socket, info)
	}
	if *wrap == "" {
		if line := defaultLine(info); line != "" {
			fmt.Fprintln(stdout, line)
		}
		return 0
	}
	cmd := exec.Command("sh", "-c", *wrap)
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	return 0
}

// Parse reads the Claude Code status line JSON.
func Parse(data []byte) (proto.StatuslineInfo, error) {
	var in struct {
		SessionID string `json:"session_id"`
		Cwd       string `json:"cwd"`
		Model     struct {
			DisplayName string `json:"display_name"`
		} `json:"model"`
		Effort        json.RawMessage `json:"effort"`
		ContextWindow struct {
			UsedPercentage float64 `json:"used_percentage"`
		} `json:"context_window"`
	}
	err := json.Unmarshal(data, &in)
	return proto.StatuslineInfo{
		SessionID:  in.SessionID,
		Cwd:        in.Cwd,
		Model:      in.Model.DisplayName,
		Effort:     effort(in.Effort),
		ContextPct: int(math.Round(in.ContextWindow.UsedPercentage)),
	}, err
}

// effort reads the effort as {"level": "high"} (Claude Code 2.1.3xx) or as a plain string (older versions).
func effort(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var o struct {
		Level string `json:"level"`
	}
	json.Unmarshal(raw, &o)
	return o.Level
}

func send(socket string, info proto.StatuslineInfo) {
	payload, err := json.Marshal(info)
	if err != nil {
		return
	}
	deadline := time.Now().Add(sendTimeout)
	conn, err := net.DialTimeout("unix", socket, sendTimeout)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetDeadline(deadline)
	proto.WriteLine(conn, proto.Request{Kind: proto.Statusline, Payload: payload})
}

func defaultLine(info proto.StatuslineInfo) string {
	var parts []string
	for _, p := range []string{info.Model, info.Effort} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if info.ContextPct > 0 {
		parts = append(parts, fmt.Sprintf("%d%% ctx", info.ContextPct))
	}
	return strings.Join(parts, " · ")
}
