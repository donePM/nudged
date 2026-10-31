package main

import (
	"os"

	"github.com/rokde/nudged/internal/proto"
	"github.com/rokde/nudged/internal/statusline"
)

func init() {
	register("statusline", func(args []string) int {
		socket, _ := proto.SocketPath()
		return statusline.Run(args, os.Stdin, os.Stdout, socket)
	})
}
