package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/donepm/nudged/internal/installer"
)

func init() {
	register("install-hooks", func(args []string) int {
		path, err := settingsPath()
		if err == nil {
			var bin string
			if bin, err = os.Executable(); err == nil {
				if bin, err = filepath.EvalSymlinks(bin); err == nil {
					err = installer.Install(path, bin)
				}
			}
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "nudged install-hooks:", err)
			return 1
		}
		fmt.Println("nudged hooks installed in", path)
		return 0
	})
}

func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json"), err
}
