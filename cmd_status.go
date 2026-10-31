package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
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

func runStatus(socket string, stdout, stderr io.Writer) int {
	conn, err := net.DialTimeout("unix", socket, time.Second)
	if err != nil {
		fmt.Fprintln(stderr, "nudged status: daemon not running")
		return 1
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	var rep proto.Reply
	var sessions []proto.Session
	if err = proto.WriteLine(conn, proto.Request{Kind: proto.Status}); err == nil {
		err = proto.ReadLine(bufio.NewReader(conn), &rep)
	}
	if err == nil && !rep.OK {
		err = fmt.Errorf("daemon error: %s", rep.Error)
	}
	if err == nil {
		err = json.Unmarshal(rep.Output, &sessions)
	}
	if err != nil {
		fmt.Fprintln(stderr, "nudged status:", err)
		return 1
	}
	printSessions(stdout, sessions, time.Now())
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
