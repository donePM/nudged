package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/rokde/nudged/internal/proto"
)

func init() {
	register("answer", func(args []string) int {
		socket, err := proto.SocketPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, "nudged answer:", err)
			return 1
		}
		return runAnswer(socket, args, os.Stdout, os.Stderr)
	})
}

// runAnswer sends the JSON args[1] as the answer to the held request args[0]. It never prints the payload.
func runAnswer(socket string, args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "nudged answer: usage: nudged answer <id> <json>")
		return 1
	}
	if !json.Valid([]byte(args[1])) {
		fmt.Fprintln(stderr, "nudged answer: answer is not valid JSON")
		return 1
	}
	rep, err := query(socket, proto.Request{Kind: proto.Answer, ID: args[0], Payload: json.RawMessage(args[1])})
	if err == nil && !rep.OK {
		err = errors.New(rep.Error)
	}
	if err != nil {
		fmt.Fprintln(stderr, "nudged answer:", err)
		return 1
	}
	return 0
}
