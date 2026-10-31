package main

import (
	"fmt"
	"os"

	"github.com/rokde/nudged/internal/sound"
)

func init() {
	register("sound", func(args []string) int {
		if len(args) != 1 {
			args = []string{""}
		}
		if _, err := sound.WAV(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, "usage: nudged sound <permission|question|notice>")
			return 2
		}
		if err := sound.Play(args[0]); err != nil {
			fmt.Fprintln(os.Stderr, "nudged sound:", err)
			return 1
		}
		return 0
	})
}
