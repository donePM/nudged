package main

import (
	"flag"
	"io"

	"github.com/donepm/nudged/internal/focus"
)

func init() {
	register("focus", func(args []string) int {
		fs := flag.NewFlagSet("focus", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		tty := fs.String("tty", "", "TTY device path")
		term := fs.String("term", "", "TERM_PROGRAM value")
		if fs.Parse(args) == nil {
			focus.Jump(*tty, *term)
		}
		return 0
	})
}
