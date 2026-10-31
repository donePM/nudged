package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/donepm/nudged/internal/config"
)

func TestReadmeTexts(t *testing.T) {
	fileContains(t, "README.md",
		"brew install --cask donepm/tap/nudged", "xattr -dr com.apple.quarantine", "nudged install-hooks",
		"nudged uninstall-hooks", "make build",
		"⌥⌘P", "~/.config/nudged/config.toml", "Session done", "Session waits", "notify", "answer",
		"Start at login", "MIT")
}

func TestReadmeSubcommands(t *testing.T) {
	want := make([]string, 0, len(commands))
	for name := range commands {
		want = append(want, "nudged "+name)
	}
	fileContains(t, "README.md", want...)
}

func TestReadmeConfig(t *testing.T) {
	s := fileContains(t, "README.md")
	_, block, ok := strings.Cut(s, "```toml\n")
	if block, _, ok = strings.Cut(block, "```"); !ok {
		t.Fatal("README.md has no toml block")
	}
	var got config.Config
	md, err := toml.Decode(block, &got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, config.Default()) {
		t.Errorf("toml block decodes to %+v, want %+v", got, config.Default())
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(config.Default()); err != nil {
		t.Fatal(err)
	}
	var back config.Config
	want, err := toml.Decode(buf.String(), &back)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range want.Keys() {
		if !md.IsDefined(k...) {
			t.Errorf("toml block has no key %s", k)
		}
	}
}
