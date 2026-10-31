package main

import (
	"fmt"
	"os"

	"github.com/donepm/nudged/internal/installer"
)

func init() {
	register("uninstall-hooks", func(args []string) int {
		path, err := settingsPath()
		if err == nil {
			err = installer.Uninstall(path)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "nudged uninstall-hooks:", err)
			return 1
		}
		fmt.Println("nudged hooks removed from", path)
		return 0
	})
}
