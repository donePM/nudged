// Package focus brings the terminal window of a TTY to the front.
package focus

import (
	"os/exec"
	"regexp"
)

// validTTY keeps the TTY safe to put in an AppleScript string.
var validTTY = regexp.MustCompile(`^/dev/[A-Za-z0-9]+$`)

// Script returns the AppleScript that shows the TTY in the terminal term.
// It returns false if no script fits.
func Script(tty, term string) (string, bool) {
	if term == "ghostty" {
		// shortcut: Ghostty has no TTY match in AppleScript, upgrade when it gets one
		return `tell application "Ghostty" to activate`, true
	}
	if !validTTY.MatchString(tty) {
		return "", false
	}
	q := `"` + tty + `"`
	switch term {
	case "Apple_Terminal":
		return `tell application "Terminal"
	repeat with w in windows
		repeat with t in tabs of w
			if tty of t is ` + q + ` then
				set selected of t to true
				set index of w to 1
				activate
				return
			end if
		end repeat
	end repeat
end tell`, true
	case "iTerm.app":
		return `tell application "iTerm"
	repeat with w in windows
		repeat with t in tabs of w
			repeat with s in sessions of t
				if tty of s is ` + q + ` then
					select w
					select t
					select s
					activate
					return
				end if
			end repeat
		end repeat
	end repeat
end tell`, true
	}
	return "", false
}

// Jump runs the script for the TTY with osascript. It ignores all errors.
func Jump(tty, term string) {
	if s, ok := Script(tty, term); ok {
		_ = exec.Command("osascript", "-e", s).Run()
	}
}
