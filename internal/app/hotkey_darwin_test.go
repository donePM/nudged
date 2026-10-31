//go:build darwin

package app

import "testing"

func TestHotkeyConstants(t *testing.T) {
	if hotkeyCode != 0x23 {
		t.Fatalf("key code %#x", hotkeyCode)
	}
	if hotkeyMods != 0x0100|0x0800 {
		t.Fatalf("modifiers %#x", hotkeyMods)
	}
}
