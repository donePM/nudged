package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/donepm/nudged/internal/config"
	"github.com/donepm/nudged/internal/daemon"
	"github.com/donepm/nudged/internal/installer"
	"github.com/donepm/nudged/internal/loginitem"
	"github.com/donepm/nudged/internal/sound"
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
var newSettingsWindow = func(app *application.App, srv *daemon.Server, u *updater) settingsWindow {
	registerSettingsEvents(app, srv, u)
	w := app.Window.NewWithOptions(settingsOptions())
	// The close button hides the window, so the next open shows the same window.
	w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		w.Hide()
		e.Cancel()
	})
	return webWindow{w}
}

// settingsOptions are the options of the settings window: a sidebar window with a hidden title bar.
func settingsOptions() application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Name:           "settings",
		Title:          "nudged Settings",
		Width:          680,
		Height:         520,
		URL:            settingsURL,
		DisableResize:  true,
		BackgroundType: application.BackgroundTypeTranslucent,
		Mac: application.MacWindow{
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 52,
		},
	}
}

var (
	settingsMu   sync.Mutex
	settingsWin  settingsWindow
	settingsPage string // the page the next settings:get opens, for a window whose view is not loaded yet
)

// emitSettings sends an event to the settings view. Tests replace it.
var emitSettings = func(app *application.App, name string, data any) { app.Event.Emit(name, data) }

// openSettings creates the settings window on the first call. Every call reloads config.toml into the view, then shows and focuses the window.
// A page other than "" ("about", for example) opens that page of the view.
func openSettings(app *application.App, srv *daemon.Server, u *updater, page string) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	if settingsWin == nil {
		settingsWin = newSettingsWindow(app, srv, u)
	}
	cfg, err := loadSettings()
	emitSettings(app, "settings:config", cfg)
	if err != nil {
		emitSettings(app, "settings:error", err.Error())
	}
	if page != "" {
		settingsPage = page
		emitSettings(app, "settings:page", page)
	}
	settingsWin.show()
}

// takeSettingsPage returns the page of the last openSettings call once.
func takeSettingsPage() string {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	p := settingsPage
	settingsPage = ""
	return p
}

type hooksResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func registerSettingsEvents(app *application.App, srv *daemon.Server, u *updater) {
	emit := func(name string, data any) { app.Event.Emit(name, data) }
	app.Event.On("settings:update-get", func(*application.CustomEvent) { emit("settings:update", u.state()) })
	app.Event.On("settings:update-check", func(*application.CustomEvent) { go u.check(context.Background()) })
	app.Event.On("settings:update-install", func(*application.CustomEvent) { go installUpdate(app, srv, u) })
	app.Event.On("settings:get", func(*application.CustomEvent) {
		cfg, err := loadSettings()
		emit("settings:config", cfg)
		if err != nil {
			emit("settings:error", err.Error())
		}
		if p := takeSettingsPage(); p != "" {
			emit("settings:page", p)
		}
		emit("settings:update", u.state())
		emit("settings:hooks-state", hooksInstalled())
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
		emit("settings:hooks-state", hooksInstalled())
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

// saveSettings validates cfg, sets the login item, writes cfg to config.toml, applies the timeout to the running daemon, and sends cfg to the panel loop.
func saveSettings(srv *daemon.Server, cfg config.Config) error {
	if cfg.Mode != "answer" && cfg.Mode != "notify" {
		return fmt.Errorf("mode %q is not \"answer\" or \"notify\"", cfg.Mode)
	}
	if !config.ValidPosition(cfg.Position) {
		return fmt.Errorf("position %q is not one of %s", cfg.Position, strings.Join(config.Positions, ", "))
	}
	if cfg.TimeoutMinutes <= 0 {
		return errors.New("timeout must be more than 0 minutes")
	}
	if cfg.TimeoutMinutes > config.MaxTimeoutMinutes {
		return fmt.Errorf("timeout must be %d minutes or less", config.MaxTimeoutMinutes)
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
	for sent := false; !sent; {
		select {
		case panelConfigs <- cfg:
			sent = true
		default:
			select { // drop the old config, so the newest one wins
			case <-panelConfigs:
			default:
			}
		}
	}
	return nil
}

// panelConfigs takes the saved config to the panel loop.
var panelConfigs = make(chan config.Config, 1)

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

var (
	loginEnabled = loginitem.Enabled
	loginSet     = setLoginItem
)

// syncLoginItem makes the LaunchAgent plist match cfg.StartAtLogin. On error it writes one line to logw.
func syncLoginItem(cfg config.Config) {
	if cfg.StartAtLogin == loginEnabled() {
		return
	}
	if err := loginSet(cfg.StartAtLogin); err != nil {
		fmt.Fprintln(logw, "nudged: start at login:", err)
	}
}

// hooksInstalled reports whether ~/.claude/settings.json has the nudged hooks.
func hooksInstalled() bool {
	home, err := os.UserHomeDir()
	return err == nil && installer.Installed(filepath.Join(home, ".claude", "settings.json"))
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
