package main

import (
	"fmt"
	"io"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/rokde/nudged/internal/config"
)

func init() {
	register("config", func(args []string) int { return printConfig(os.Stdout, os.Stderr) })
}

func printConfig(stdout, stderr io.Writer) int {
	path, err := config.Path()
	if err == nil {
		var cfg config.Config
		if cfg, err = config.Load(); err == nil {
			fmt.Fprintln(stdout, path)
			err = toml.NewEncoder(stdout).Encode(cfg)
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "nudged config:", err)
		return 1
	}
	return 0
}
