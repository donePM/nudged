// Package usage sums the Claude Code token usage and the approximate cost of one day.
package usage

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Totals holds the token sums and the approximate cost in USD.
type Totals struct {
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	CostUSD             float64
}

type line struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"requestId"`
	Message   struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			Input         int64 `json:"input_tokens"`
			Output        int64 `json:"output_tokens"`
			CacheCreation int64 `json:"cache_creation_input_tokens"`
			CacheRead     int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// Today sums the usage of all assistant lines in root/**/*.jsonl on the calendar day of now, in the location of now.
// A missing root gives zero totals. Unreadable files and malformed lines are skipped.
func Today(root string, now time.Time) (Totals, error) {
	var t Totals
	y, m, d := now.Date()
	start := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			return nil
		}
		if e.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		// A file with lines of today has a modification time of today or later.
		if info, err := e.Info(); err != nil || info.ModTime().Before(start) {
			return nil
		}
		addFile(&t, path, now, seen)
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return Totals{}, nil
	}
	return t, err
}

func addFile(t *Totals, path string, now time.Time, seen map[string]bool) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	// Lines with tool results can be large.
	sc.Buffer(make([]byte, 0, 1<<20), 256<<20)
	y, m, d := now.Date()
	for sc.Scan() {
		var l line
		if json.Unmarshal(sc.Bytes(), &l) != nil || l.Type != "assistant" || l.Message.Usage == nil {
			continue
		}
		if ly, lm, ld := l.Timestamp.In(now.Location()).Date(); ly != y || lm != m || ld != d {
			continue
		}
		if l.Message.ID != "" || l.RequestID != "" {
			key := l.Message.ID + "\x00" + l.RequestID
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		u := l.Message.Usage
		t.InputTokens += u.Input
		t.OutputTokens += u.Output
		t.CacheCreationTokens += u.CacheCreation
		t.CacheReadTokens += u.CacheRead
		if p, ok := priceFor(l.Message.Model); ok {
			t.CostUSD += (float64(u.Input)*p.input + float64(u.Output)*p.output +
				float64(u.CacheCreation)*p.input*1.25 + float64(u.CacheRead)*p.input*0.1) / 1e6
		}
	}
}
