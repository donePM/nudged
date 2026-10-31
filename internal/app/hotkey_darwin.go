//go:build darwin

package app

/*
#cgo LDFLAGS: -framework Carbon
#include <Carbon/Carbon.h>

extern void nudgedHotkeyPressed(void);

static OSStatus nudgedHotkeyHandler(EventHandlerCallRef next, EventRef ev, void *data) {
	nudgedHotkeyPressed();
	return noErr;
}

static OSStatus nudgedRegisterHotkey(UInt32 code, UInt32 mods) {
	EventTypeSpec spec = {kEventClassKeyboard, kEventHotKeyPressed};
	OSStatus st = InstallEventHandler(GetApplicationEventTarget(), NewEventHandlerUPP(nudgedHotkeyHandler), 1, &spec, NULL, NULL);
	if (st != noErr) return st;
	EventHotKeyID id = {'nudg', 1};
	EventHotKeyRef ref;
	return RegisterEventHotKey(code, mods, id, GetApplicationEventTarget(), 0, &ref);
}
*/
import "C"

import "fmt"

const (
	hotkeyCode = C.kVK_ANSI_P
	hotkeyMods = C.cmdKey | C.optionKey
)

var hotkeyPress func()

//export nudgedHotkeyPressed
func nudgedHotkeyPressed() {
	if hotkeyPress != nil {
		hotkeyPress()
	}
}

func init() {
	registerHotkey = func(onPress func()) error {
		hotkeyPress = onPress
		if st := C.nudgedRegisterHotkey(hotkeyCode, hotkeyMods); st != C.noErr {
			return fmt.Errorf("register ⌥⌘P: OSStatus %d", int(st))
		}
		return nil
	}
}
