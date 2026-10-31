// Package sound synthesizes the alert tones as WAV data and plays them.
package sound

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
)

const rate = 44100

// player is a variable so that tests can use a fake player.
var player = "afplay"

type tone struct {
	seconds float64
	sample  func(t, d float64) float64 // t and d in seconds, result in [-1, 1]
}

var tones = map[string]tone{
	// two soft rising tones
	"permission": {0.40, func(t, d float64) float64 {
		f, start := 660.0, 0.0
		if t >= d/2 {
			f, start = 880.0, d/2
		}
		return 0.3 * math.Sin(math.Pi*(t-start)/(d/2)) * math.Sin(2*math.Pi*f*t)
	}},
	// a soft bell: a fundamental and a quiet overtone that decay
	"question": {0.60, func(t, d float64) float64 {
		return 0.35 * math.Exp(-6*t) * (0.8*math.Sin(2*math.Pi*1046.5*t) + 0.2*math.Sin(2*math.Pi*2093*t))
	}},
	// a short low tone
	"notice": {0.20, func(t, d float64) float64 {
		return 0.35 * math.Sin(2*math.Pi*330*t)
	}},
}

// WAV returns the named tone as a PCM, 16-bit, mono, 44100 Hz WAV file.
func WAV(name string) ([]byte, error) {
	tn, ok := tones[name]
	if !ok {
		return nil, fmt.Errorf("unknown sound %q", name)
	}
	n := int(math.Round(rate * tn.seconds))
	fade := float64(rate) * 0.005 // 5 ms fade at each end prevents a click
	data := make([]int16, n)
	for i := range data {
		env := math.Min(1, math.Min(float64(i), float64(n-1-i))/fade)
		v := env * tn.sample(float64(i)/rate, tn.seconds)
		data[i] = int16(math.Round(math.Max(-0.5, math.Min(0.5, v)) * 32767))
	}
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, struct {
		Riff                  [4]byte
		Size                  uint32
		Wave, Fmt             [4]byte
		FmtSize               uint32
		Format, Channels      uint16
		Rate, ByteRate        uint32
		BlockAlign, BitsPerSm uint16
		Data                  [4]byte
		DataSize              uint32
	}{
		[4]byte{'R', 'I', 'F', 'F'}, uint32(36 + 2*n), [4]byte{'W', 'A', 'V', 'E'}, [4]byte{'f', 'm', 't', ' '},
		16, 1, 1, rate, rate * 2, 2, 16, [4]byte{'d', 'a', 't', 'a'}, uint32(2 * n),
	})
	binary.Write(&b, binary.LittleEndian, data)
	return b.Bytes(), nil
}

// Play writes the named tone to a temp file and plays it with afplay until it stops.
func Play(name string) error {
	wav, err := WAV(name)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "nudged-*.wav")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(wav); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return exec.Command(player, f.Name()).Run()
}
