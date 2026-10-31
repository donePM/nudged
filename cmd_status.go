package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"text/tabwriter"
	"time"

	"github.com/rokde/nudged/internal/proto"
)

func init() {
	register("status", func(args []string) int {
		socket, err := proto.SocketPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, "nudged status:", err)
			return 1
		}
		return runStatus(socket, os.Stdout, os.Stderr)
	})
}

var errNoDaemon = errors.New("daemon not running")

// query sends one request on a new connection and reads one reply. The server reads one request for each connection.
func query(socket string, req proto.Request) (proto.Reply, error) {
	var rep proto.Reply
	conn, err := net.DialTimeout("unix", socket, time.Second)
	if err != nil {
		return rep, errNoDaemon
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err = proto.WriteLine(conn, req); err == nil {
		err = proto.ReadLine(bufio.NewReader(conn), &rep)
	}
	return rep, err
}

// list queries kind and decodes the reply output into v.
func list(socket, kind string, v any) error {
	rep, err := query(socket, proto.Request{Kind: kind})
	if err == nil && !rep.OK {
		err = fmt.Errorf("daemon error: %s", rep.Error)
	}
	if err == nil {
		err = json.Unmarshal(rep.Output, v)
	}
	return err
}

func runStatus(socket string, stdout, stderr io.Writer) int {
	var sessions []proto.Session
	var pending []proto.PendingRequest
	err := list(socket, proto.Status, &sessions)
	if err == nil {
		err = list(socket, proto.Pending, &pending)
	}
	if err != nil {
		fmt.Fprintln(stderr, "nudged status:", err)
		return 1
	}
	now := time.Now()
	printSessions(stdout, sessions, now)
	printPending(stdout, pending, sessions, now)
	return 0
}

func printSessions(w io.Writer, sessions []proto.Session, now time.Time) {
	if len(sessions) == 0 {
		fmt.Fprintln(w, "no sessions")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, s := range sessions {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d%%\t%s\t%s\t%s\t%s\n", s.Status, s.Project, s.Model, s.Effort,
			s.ContextPct, s.Term, s.TTY, now.Sub(s.LastActivity).Round(time.Second), s.Path)
	}
	tw.Flush()
}

// printPending prints the held requests, oldest first. It prints nothing when there are none.
func printPending(w io.Writer, pending []proto.PendingRequest, sessions []proto.Session, now time.Time) {
	if len(pending) == 0 {
		return
	}
	fmt.Fprint(w, "\npending\n")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, p := range pending {
		project := p.SessionID
		if i := slices.IndexFunc(sessions, func(s proto.Session) bool { return s.ID == p.SessionID }); i >= 0 {
			project = sessions[i].Project
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.ID, p.Kind, project, now.Sub(p.Started).Round(time.Second))
	}
	tw.Flush()
}
