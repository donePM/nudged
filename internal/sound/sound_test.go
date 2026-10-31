package sound

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var durations = map[string]float64{"permission": 0.40, "question": 0.60, "notice": 0.20}

func TestHeader(t *testing.T) {
	for name := range durations {
		b, err := WAV(name)
		if err != nil {
			t.Fatal(err)
		}
		le := binary.LittleEndian
		if string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" || string(b[12:16]) != "fmt " || string(b[36:40]) != "data" {
			t.Fatalf("%s: bad chunk ids %q", name, b[:44])
		}
		if got := le.Uint32(b[4:8]); int(got) != len(b)-8 {
			t.Errorf("%s: RIFF size %d, len %d", name, got, len(b))
		}
		if le.Uint32(b[16:20]) != 16 || le.Uint16(b[20:22]) != 1 || le.Uint16(b[22:24]) != 1 ||
			le.Uint32(b[24:28]) != 44100 || le.Uint32(b[28:32]) != 88200 || le.Uint16(b[32:34]) != 2 || le.Uint16(b[34:36]) != 16 {
			t.Errorf("%s: bad fmt chunk % x", name, b[16:36])
		}
	}
}

func TestLength(t *testing.T) {
	seen := map[string]string{}
	for name, d := range durations {
		b, _ := WAV(name)
		want := int(math.Round(44100*d)) * 2
		if got := binary.LittleEndian.Uint32(b[40:44]); int(got) != want || len(b) != 44+want {
			t.Errorf("%s: data size %d, len %d, want %d", name, got, len(b), want)
		}
		if other, ok := seen[string(b)]; ok {
			t.Errorf("%s and %s are identical", name, other)
		}
		seen[string(b)] = name
	}
}

func TestSamples(t *testing.T) {
	for name := range durations {
		b, _ := WAV(name)
		s := make([]int16, (len(b)-44)/2)
		binary.Read(bytes.NewReader(b[44:]), binary.LittleEndian, s)
		nonzero := false
		for i, v := range s {
			if v > 16384 || v < -16384 {
				t.Fatalf("%s: sample %d is %d", name, i, v)
			}
			nonzero = nonzero || v != 0
		}
		if !nonzero || s[0] != 0 || s[len(s)-1] != 0 {
			t.Errorf("%s: nonzero %v, first %d, last %d", name, nonzero, s[0], s[len(s)-1])
		}
	}
}

func TestUnknown(t *testing.T) {
	if _, err := WAV("other"); err == nil {
		t.Error("WAV: no error")
	}
	old := player
	player = "/nonexistent-player"
	defer func() { player = old }()
	if err := Play("other"); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("Play: %v", err)
	}
}

func TestPlay(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "player")
	log := filepath.Join(dir, "log")
	copied := filepath.Join(dir, "copy")
	os.WriteFile(script, []byte("#!/bin/sh\necho \"$@\" >> '"+log+"'\ncat \"$1\" > '"+copied+"'\n"), 0o700)
	old := player
	player = script
	defer func() { player = old }()
	if err := Play("question"); err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(log)
	lines := strings.Split(strings.TrimSpace(string(calls)), "\n")
	if len(lines) != 1 {
		t.Fatalf("player calls %q", calls)
	}
	got, _ := os.ReadFile(copied)
	want, _ := WAV("question")
	if !bytes.Equal(got, want) {
		t.Error("player file differs from WAV(question)")
	}
	if _, err := os.Stat(lines[0]); !os.IsNotExist(err) {
		t.Errorf("temp file %s still exists: %v", lines[0], err)
	}
}

func TestPlayError(t *testing.T) {
	old := player
	player = filepath.Join(t.TempDir(), "missing")
	defer func() { player = old }()
	if err := Play("notice"); err == nil {
		t.Error("no error")
	}
}
