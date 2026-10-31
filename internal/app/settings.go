package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rokde/nudged/internal/config"
	"github.com/rokde/nudged/internal/daemon"
	"github.com/rokde/nudged/internal/installer"
	"github.com/rokde/nudged/internal/loginitem"
	"github.com/rokde/nudged/internal/sound"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const settingsURL = "/#/Settings"

type settingsWindow interface{ show() }

type webWindow struct{ w *application.WebviewWindow }

func (w webWindow) show() {
	w.w.Show()
	w.w.Focus()
}

// newSettingsWindow registers the settings events and creates the window. Tests replace it.
var newSettingsWindow = func(app *application.App, srv *daemon.Server) settingsWindow {
	registerSettingsEvents(app, srv)
	w := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   "settings",
		Title:  "nudged Settings",
		Width:  560,
		Height: 480,
		URL:    settingsURL,
	})
	// The close button hides the window, so the next open shows the same window.
	w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		w.Hide()
		e.Cancel()
	})
	return webWindow{w}
}

var (
	settingsMu  sync.Mutex
	settingsWin settingsWindow
)

// emitSettings sends an event to the settings view. Tests replace it.
var emitSettings = func(app *application.App, name string, data any) { app.Event.Emit(name, data) }

// openSettings creates the settings window on the first call. Every call reloads config.toml into the view, then shows and focuses the window.
func openSettings(app *application.App, srv *daemon.Server) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	if settingsWin == nil {
		settingsWin = newSettingsWindow(app, srv)
	}
	cfg, err := loadSettings()
	emitSettings(app, "settings:config", cfg)
	if err != nil {
		emitSettings(app, "settings:error", err.Error())
	}
	settingsWin.show()
}

type hooksResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func registerSettingsEvents(app *application.App, srv *daemon.Server) {
	emit := func(name string, data any) { app.Event.Emit(name, data) }
	app.Event.On("settings:get", func(*application.CustomEvent) {
		cfg, err := loadSettings()
		emit("settings:config", cfg)
		if err != nil {
			emit("settings:error", err.Error())
		}
	})
	app.Event.On("settings:save", func(e *application.CustomEvent) {
		cfg, err := decodeConfig(e.Data)
		if err == nil {
			err = saveSettings(srv, cfg)
		}
		if err != nil {
			emit("settings:error", err.Error())
			return
		}
		emit("settings:saved", cfg)
	})
	app.Event.On("settings:sound", func(e *application.CustomEvent) {
		key, _ := e.Data.(string)
		if err := testSound(key); err != nil {
			emit("settings:error", err.Error())
		}
	})
	app.Event.On("settings:hooks", func(e *application.CustomEvent) {
		action, _ := e.Data.(string)
		msg, err := runHooks(action)
		if err != nil {
			emit("settings:hooks-result", hooksResult{Message: err.Error()})
			return
		}
		emit("settings:hooks-result", hooksResult{OK: true, Message: msg})
	})
}

// loadSettings loads the config and sets StartAtLogin from the plist, so the view shows the real state.
func loadSettings() (config.Config, error) {
	cfg, err := config.Load()
	cfg.StartAtLogin = loginitem.Enabled()
	return cfg, err
}

// decodeConfig reads the JSON event data on top of the current config, so fields the view does not send stay.
func decodeConfig(data any) (config.Config, error) {
	cfg, _ := config.Load()
	b, err := json.Marshal(data)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(b, &cfg)
	return cfg, err
}

// saveSettings validates cfg, sets the login item, writes cfg to config.toml, and applies the timeout to the running daemon.
func saveSettings(srv *daemon.Server, cfg config.Config) error {
	if cfg.Mode != "answer" && cfg.Mode != "notify" {
		return fmt.Errorf("mode %q is not \"answer\" or \"notify\"", cfg.Mode)
	}
	if cfg.TimeoutMinutes <= 0 {
		return errors.New("timeout must be more than 0 minutes")
	}
	if cfg.TimeoutMinutes > 1440 {
		return errors.New("timeout must be 1440 minutes (24 h) or less")
	}
	e := cfg.Events
	for name, ev := range map[string]config.Event{"permission": e.Permission, "question": e.Question, "done": e.Done, "waits": e.Waits} {
		if ev.HideSeconds < 0 {
			return fmt.Errorf("hide seconds of %s must be 0 or more", name)
		}
	}
	if err := setLoginItem(cfg.StartAtLogin); err != nil {
		return err
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	srv.SetHoldTimeout(time.Duration(cfg.TimeoutMinutes) * time.Minute)
	return nil
}

var play = sound.Play

var soundNames = map[string]string{"permission": "permission", "question": "question", "done": "notice", "waits": "notice"}

// testSound plays the sound of the event key in a goroutine, so the caller does not wait.
func testSound(key string) error {
	name, ok := soundNames[key]
	if !ok {
		return fmt.Errorf("unknown event %q", key)
	}
	go func() {
		if err := play(name); err != nil {
			fmt.Fprintln(logw, "nudged: sound:", err)
		}
	}()
	return nil
}

var executable = os.Executable

// binPath returns the absolute path of the running binary with symlinks resolved.
func binPath() (string, error) {
	bin, err := executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(bin)
}

func setLoginItem(on bool) error {
	if !on {
		return loginitem.Disable()
	}
	bin, err := binPath()
	if err != nil {
		return err
	}
	return loginitem.Enable(bin)
}

// runHooks installs or removes the nudged hooks in ~/.claude/settings.json, as nudged install-hooks and uninstall-hooks do.
func runHooks(action string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, ".claude", "settings.json")
	switch action {
	case "install":
		bin, err := binPath()
		if err == nil {
			err = installer.Install(path, bin)
		}
		return "Hooks installed in " + path, err
	case "remove":
		return "Hooks removed from " + path, installer.Uninstall(path)
	}
	return "", fmt.Errorf("unknown hooks action %q", action)
}
