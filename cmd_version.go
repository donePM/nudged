package main

import (
	"fmt"
	"io"
	"os"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func init() {
	register("version", func(args []string) int { return printVersion(os.Stdout) })
}

func printVersion(w io.Writer) int {
	fmt.Fprintf(w, "nudged %s\n", version)
	return 0
}
