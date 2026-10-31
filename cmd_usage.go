package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	usagelog "github.com/donepm/nudged/internal/usage"
)

func init() {
	register("usage", func(args []string) int { return printUsage(os.Stdout) })
}

func printUsage(w io.Writer) int {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "nudged usage:", err)
		return 1
	}
	t, err := usagelog.Today(filepath.Join(home, ".claude", "projects"), time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, "nudged usage:", err)
		return 1
	}
	fmt.Fprintf(w, "input        %d\noutput       %d\ncache write  %d\ncache read   %d\ncost         ≈ $%.2f\n",
		t.InputTokens, t.OutputTokens, t.CacheCreationTokens, t.CacheReadTokens, t.CostUSD)
	return 0
}
