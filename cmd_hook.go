package main

import (
	"os"

	"github.com/donepm/nudged/internal/hookclient"
	"github.com/donepm/nudged/internal/proto"
)

func init() {
	register("hook", func(args []string) int {
		socket, err := proto.SocketPath()
		if err != nil || len(args) == 0 {
			return 0
		}
		return hookclient.Run(args[0], os.Stdin, os.Stdout, socket, os.Getenv("TERM_PROGRAM"))
	})
}
