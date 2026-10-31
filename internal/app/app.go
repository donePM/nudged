// Package app is the tray app. It runs the daemon in the same process.
package app

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/rokde/nudged/frontend"
	"github.com/rokde/nudged/internal/config"
	"github.com/rokde/nudged/internal/daemon"
	"github.com/rokde/nudged/internal/proto"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed running.png
var runningIcon []byte

//go:embed paused.png
var pausedIcon []byte

type menuItem struct{ Label, Accelerator string }

var menuItems = []menuItem{
	{"Pause", "Option+CmdOrCtrl+P"},
	{"Settings…", "CmdOrCtrl+,"},
	{"Quit", "CmdOrCtrl+Q"},
}

// registerHotkey registers ⌥⌘P and calls onPress for each key press. hotkey_darwin.go sets it.
var registerHotkey = func(onPress func()) error { return errors.New("hotkey not supported") }

var logw io.Writer = os.Stderr

// shortcut: no settings window, S19 sets this func
var openSettings = func() {}

// startDaemon listens on socket and serves in the background. The channel gets the error of Serve.
func startDaemon(socket string) (*daemon.Server, <-chan error, error) {
	srv, err := daemon.Listen(socket, daemon.NewRegistry())
	if err != nil {
		return nil, nil, err
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "nudged:", err)
	}
	if cfg.TimeoutMinutes > 0 {
		srv.HoldTimeout = time.Duration(cfg.TimeoutMinutes) * time.Minute
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve() }()
	return srv, errc, nil
}

// assets serves the Vite output in frontend/dist/app. Without a build, it serves the placeholder frontend/dist/index.html.
func assets() http.Handler {
	root, _ := fs.Sub(frontend.Dist, "dist")
	if _, err := fs.Stat(root, "app/index.html"); err == nil {
		root, _ = fs.Sub(root, "app")
	}
	return application.AssetFileServerFS(root)
}

// toggle flips the pause flag. It returns the new tray icon and the new label of the Pause item.
func toggle(srv *daemon.Server) ([]byte, string) {
	paused := !srv.Paused()
	srv.SetPaused(paused)
	if paused {
		return pausedIcon, "Resume"
	}
	return runningIcon, menuItems[0].Label
}

// applyToggle flips the pause flag and shows the new state in the tray. The Pause click and the hotkey both call it.
func applyToggle(srv *daemon.Server, setIcon func([]byte), setLabel func(string)) {
	icon, label := toggle(srv)
	setIcon(icon)
	setLabel(label)
}

// setupHotkey registers the pause hotkey. On error it writes one line to logw, and the Pause item stays usable.
func setupHotkey(onPress func()) {
	if err := registerHotkey(onPress); err != nil {
		fmt.Fprintln(logw, "nudged: pause hotkey:", err)
	}
}

// quit closes the server, which removes the socket file, and then quits the app.
func quit(srv *daemon.Server, quitApp func()) {
	srv.Close()
	quitApp()
}

// Run starts the daemon and the tray app. It returns the exit code.
func Run() int {
	socket, err := proto.SocketPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "nudged:", err)
		return 1
	}
	return run(socket)
}

func run(socket string) int {
	srv, errc, err := startDaemon(socket)
	if err != nil {
		fmt.Fprintln(os.Stderr, "nudged:", err)
		return 1
	}
	app := application.New(application.Options{
		Name:       "nudged",
		Mac:        application.MacOptions{ActivationPolicy: application.ActivationPolicyAccessory},
		Assets:     application.AssetOptions{Handler: assets()},
		OnShutdown: func() { srv.Close() },
	})
	tray := app.SystemTray.New()
	tray.SetTemplateIcon(runningIcon)
	menu := app.NewMenu()
	var pauseItem *application.MenuItem
	pause := func() {
		application.InvokeSync(func() {
			applyToggle(srv, func(b []byte) { tray.SetTemplateIcon(b) }, func(l string) { pauseItem.SetLabel(l) })
		})
	}
	actions := []func(*application.Context){
		func(*application.Context) { pause() },
		func(*application.Context) { openSettings() },
		func(*application.Context) { quit(srv, app.Quit) },
	}
	for i, it := range menuItems {
		item := menu.Add(it.Label).SetAccelerator(it.Accelerator).OnClick(actions[i])
		if i == 0 {
			pauseItem = item
		}
	}
	tray.SetMenu(menu)
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		application.InvokeSync(func() { setupHotkey(pause) })
	})
	go func() {
		if err := <-errc; err != nil {
			fmt.Fprintln(os.Stderr, "nudged:", err)
			app.Quit()
		}
	}()
	if err := app.Run(); err != nil {
		srv.Close()
		fmt.Fprintln(os.Stderr, "nudged:", err)
		return 1
	}
	return 0
}
