package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/donepm/nudged/internal/app"
)

// startApp runs the tray app. Tests replace it.
var startApp = app.Run

var commands = map[string]func(args []string) int{}

func register(name string, fn func(args []string) int) {
	commands[name] = fn
}

func usage() {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	slices.Sort(names)
	fmt.Fprintf(os.Stderr, "usage: nudged <%s> [args]\n", strings.Join(names, "|"))
}

func run(args []string) int {
	if len(args) == 0 {
		return startApp()
	}
	fn, ok := commands[args[0]]
	if !ok {
		usage()
		return 2
	}
	return fn(args[1:])
}

func main() {
	os.Exit(run(os.Args[1:]))
}
