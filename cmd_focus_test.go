package main

import (
	"io"
	"os"
	"testing"
)

func TestFocusCommandSilent(t *testing.T) {
	for _, args := range [][]string{
		{"focus", "--tty", "/dev/ttys999", "--term", "unknown"},
		{"focus"},
		{"focus", "--bad"},
	} {
		outR, outW, _ := os.Pipe()
		errR, errW, _ := os.Pipe()
		oldOut, oldErr := os.Stdout, os.Stderr
		os.Stdout, os.Stderr = outW, errW
		code := run(args)
		os.Stdout, os.Stderr = oldOut, oldErr
		outW.Close()
		errW.Close()
		out, _ := io.ReadAll(outR)
		errOut, _ := io.ReadAll(errR)
		if code != 0 || len(out) != 0 || len(errOut) != 0 {
			t.Errorf("%v: code %d, stdout %q, stderr %q", args, code, out, errOut)
		}
	}
}
