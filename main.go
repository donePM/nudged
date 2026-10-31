package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

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
	// shortcut: no app yet, S12 starts the tray app here
	if len(args) == 0 {
		usage()
		return 2
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
