// Package hookclient forwards one Claude Code hook event to the daemon and prints its reply.
package hookclient

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/donepm/nudged/internal/proto"
)

// Run always returns 0 and writes nothing on error: exit 2 blocks Claude Code, other codes show an error.
func Run(event string, stdin io.Reader, stdout io.Writer, socket, term string) int {
	if event == "" {
		return 0
	}
	payload, err := io.ReadAll(stdin)
	if err != nil || !json.Valid(payload) {
		return 0
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return 0
	}
	defer conn.Close()
	req := proto.Request{Kind: proto.Hook, Event: event, Payload: payload, TTY: parentTTY(), Term: term}
	if proto.WriteLine(conn, req) != nil {
		return 0
	}
	var rep proto.Reply
	if proto.ReadLine(bufio.NewReader(conn), &rep) != nil {
		return 0
	}
	if rep.OK && len(rep.Output) > 0 && string(rep.Output) != "null" {
		stdout.Write(append(rep.Output, '\n'))
	}
	return 0
}

// shortcut: only the direct parent is asked, upgrade to a walk up the process tree when S10 finds sessions with no TTY.
func parentTTY() string {
	out, err := exec.Command("ps", "-o", "tty=", "-p", strconv.Itoa(os.Getppid())).Output()
	if err != nil {
		return ""
	}
	return ParseTTY(string(out))
}

// ParseTTY turns the `ps -o tty=` output into a device path, or "" when there is no TTY.
func ParseTTY(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "??" || s == "-" {
		return ""
	}
	return "/dev/" + s
}
