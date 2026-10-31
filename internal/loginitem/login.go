// Package loginitem writes and removes the LaunchAgent plist that starts nudged at login.
package loginitem

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const label = "de.donepm.nudged"

const plist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`

func path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist"), nil
}

// Enable writes the plist that starts bin at login. macOS loads it at the next login.
func Enable(bin string) error {
	p, err := path()
	if err != nil {
		return err
	}
	var esc bytes.Buffer
	if err := xml.EscapeText(&esc, []byte(bin)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, fmt.Appendf(nil, plist, label, esc.String()), 0o644)
}

// Disable removes the plist. A missing plist is not an error.
func Disable() error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Enabled reports whether the plist exists.
func Enabled() bool {
	p, err := path()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}
