// Package installer adds and removes the nudged hooks and statusline in the Claude Code settings file.
package installer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// Timeout is the hook timeout in seconds. The daemon owns the real timeout.
const Timeout = 1800

// Events are the hook events that nudged handles. PreToolUse gets the matcher AskUserQuestion.
var Events = []string{"SessionStart", "SessionEnd", "UserPromptSubmit", "PostToolUse", "Stop", "Notification", "PermissionRequest", "PreToolUse"}

// Install adds the nudged hooks and wraps the statusline. It first removes old nudged entries.
func Install(settingsPath, binPath string) error {
	return edit(settingsPath, true, func(s map[string]any) error {
		hooks, err := removeHooks(s)
		if err != nil {
			return err
		}
		if hooks == nil {
			hooks = map[string]any{}
			s["hooks"] = hooks
		}
		for _, ev := range Events {
			groups, ok := hooks[ev].([]any)
			if !ok && hooks[ev] != nil {
				return fmt.Errorf("hooks.%s is not an array", ev)
			}
			group := map[string]any{"hooks": []any{map[string]any{
				"type": "command", "command": Quote(binPath) + " hook " + ev, "timeout": Timeout,
			}}}
			if ev == "PreToolUse" {
				group["matcher"] = "AskUserQuestion"
			}
			hooks[ev] = append(groups, group)
		}

		sl, ok := s["statusLine"].(map[string]any)
		if !ok {
			if s["statusLine"] == nil {
				s["statusLine"] = map[string]any{"type": "command", "command": Quote(binPath) + " statusline"}
			}
			return nil
		}
		cmd, ok := sl["command"].(string)
		if !ok {
			return nil
		}
		if isNudged(cmd, "statusline") {
			old, wrapped, err := wrapped(cmd)
			if err != nil {
				return err
			}
			if !wrapped {
				sl["command"] = Quote(binPath) + " statusline"
				return nil
			}
			cmd = old
		}
		sl["command"] = Quote(binPath) + " statusline --wrap " + Quote(cmd)
		return nil
	})
}

// Uninstall removes all nudged hooks and restores the wrapped statusline.
func Uninstall(settingsPath string) error {
	return edit(settingsPath, false, func(s map[string]any) error {
		if _, err := removeHooks(s); err != nil {
			return err
		}
		sl, _ := s["statusLine"].(map[string]any)
		cmd, _ := sl["command"].(string)
		if !isNudged(cmd, "statusline") {
			return nil
		}
		old, ok, err := wrapped(cmd)
		if err != nil {
			return err
		}
		if ok {
			sl["command"] = old
		} else {
			delete(s, "statusLine")
		}
		return nil
	})
}

// edit reads the settings, applies change, and writes the file only when the content changed.
func edit(path string, create bool, change func(map[string]any) error) error {
	data, err := os.ReadFile(path)
	exists := err == nil
	if errors.Is(err, fs.ErrNotExist) {
		if !create {
			return nil
		}
		data = []byte("{}")
	} else if err != nil {
		return err
	}
	var before, s map[string]any
	if err := json.Unmarshal(data, &before); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if before == nil {
		return fmt.Errorf("parse %s: not a JSON object", path)
	}
	json.Unmarshal(data, &s)
	if err := change(s); err != nil {
		return err
	}
	if exists && reflect.DeepEqual(norm(before), norm(s)) {
		return nil
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		return err
	}
	if exists {
		if err := os.WriteFile(path+".nudged.bak", data, 0o600); err != nil {
			return err
		}
	} else if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// os.WriteFile keeps the mode of an existing file and follows a symlink.
	return os.WriteFile(path, buf.Bytes(), 0o600)
}

// norm makes added values (int) comparable with parsed values (float64).
func norm(v map[string]any) any {
	b, _ := json.Marshal(v)
	var out any
	json.Unmarshal(b, &out)
	return out
}

// removeHooks removes all nudged hook entries and the groups, events, and hooks key that become empty.
func removeHooks(s map[string]any) (map[string]any, error) {
	if s["hooks"] == nil {
		return nil, nil
	}
	hooks, ok := s["hooks"].(map[string]any)
	if !ok {
		return nil, errors.New("hooks is not an object")
	}
	changed := false
	for ev, v := range hooks {
		groups, ok := v.([]any)
		if !ok {
			continue
		}
		evChanged := false
		var keepGroups []any
		for _, g := range groups {
			group, ok := g.(map[string]any)
			list, _ := group["hooks"].([]any)
			if !ok || list == nil {
				keepGroups = append(keepGroups, g)
				continue
			}
			var keep []any
			for _, h := range list {
				m, _ := h.(map[string]any)
				cmd, _ := m["command"].(string)
				if !isNudged(cmd, "hook") {
					keep = append(keep, h)
				}
			}
			if len(keep) == len(list) {
				keepGroups = append(keepGroups, g)
				continue
			}
			changed, evChanged = true, true
			if len(keep) > 0 {
				group["hooks"] = keep
				keepGroups = append(keepGroups, group)
			}
		}
		if !evChanged {
			continue
		}
		if len(keepGroups) == 0 {
			delete(hooks, ev)
		} else {
			hooks[ev] = keepGroups
		}
	}
	if changed && len(hooks) == 0 {
		delete(s, "hooks")
		return nil, nil
	}
	return hooks, nil
}

// isNudged reports whether the first word of cmd has the base name nudged and the second word is sub.
func isNudged(cmd, sub string) bool {
	w, err := words(cmd)
	return err == nil && len(w) >= 2 && filepath.Base(w[0]) == "nudged" && w[1] == sub
}

// wrapped returns the --wrap value of a nudged statusline command.
func wrapped(cmd string) (string, bool, error) {
	w, err := words(cmd)
	if err != nil {
		return "", false, err
	}
	for i := 2; i+1 < len(w); i++ {
		if w[i] == "--wrap" {
			return w[i+1], true, nil
		}
	}
	return "", false, nil
}

// Quote puts s in single quotes for a POSIX shell.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// words splits a shell command into words. It knows single quotes, double quotes, and backslashes.
func words(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	inWord := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n':
			if inWord {
				out = append(out, cur.String())
				cur.Reset()
				inWord = false
			}
		case c == '\'':
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				return nil, errors.New("unterminated single quote")
			}
			cur.WriteString(s[i+1 : i+1+j])
			i += j + 1
			inWord = true
		case c == '"':
			i++
			for ; i < len(s) && s[i] != '"'; i++ {
				if s[i] == '\\' && i+1 < len(s) && strings.IndexByte("\\\"$`\n", s[i+1]) >= 0 {
					i++
				}
				cur.WriteByte(s[i])
			}
			if i >= len(s) {
				return nil, errors.New("unterminated double quote")
			}
			inWord = true
		case c == '\\':
			if i+1 < len(s) {
				i++
				cur.WriteByte(s[i])
			}
			inWord = true
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	if inWord {
		out = append(out, cur.String())
	}
	return out, nil
}
