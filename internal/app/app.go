// Package app is the tray app. It runs the daemon in the same process.
package app

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/donepm/nudged/frontend"
	"github.com/donepm/nudged/internal/config"
	"github.com/donepm/nudged/internal/daemon"
	"github.com/donepm/nudged/internal/proto"
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
	{"About nudged", ""},
	{"Quit nudged", "CmdOrCtrl+Q"},
}

// registerHotkey registers ⌥⌘P and calls onPress for each key press. hotkey_darwin.go sets it.
var registerHotkey = func(onPress func()) error { return errors.New("hotkey not supported") }

var logw io.Writer = os.Stderr

// startDaemon listens on socket and serves in the background. It returns the loaded config. The channel gets the error of Serve.
func startDaemon(socket string) (*daemon.Server, config.Config, <-chan error, error) {
	srv, cfg, err := daemon.Start(socket, "nudged:")
	if err != nil {
		return nil, config.Config{}, nil, err
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve() }()
	return srv, cfg, errc, nil
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
	srv, cfg, errc, err := startDaemon(socket)
	if err != nil {
		fmt.Fprintln(os.Stderr, "nudged:", err)
		return 1
	}
	syncLoginItem(cfg)
	app := application.New(application.Options{
		Name:        "nudged",
		Description: aboutText(),
		Mac:         application.MacOptions{ActivationPolicy: application.ActivationPolicyAccessory},
		Assets:      application.AssetOptions{Handler: assets()},
		OnShutdown:  func() { srv.Close() },
	})
	tray := app.SystemTray.New()
	tray.SetTemplateIcon(runningIcon)
	menu := app.NewMenu()
	// The update item stays hidden until a check finds a newer release.
	updateItem := menu.Add("").SetHidden(true)
	updates := newUpdater(func(i updateInfo) {
		application.InvokeAsync(func() {
			updateItem.SetLabel(updateLabel(i)).SetHidden(!i.Update).SetEnabled(!i.Checking)
		})
		app.Event.Emit("settings:update", i)
	})
	updateItem.OnClick(func(*application.Context) { go installUpdate(app, srv, updates) })
	menu.AddSeparator()
	updateCtx, stopUpdates := context.WithCancel(context.Background())
	defer stopUpdates()
	go runUpdateChecks(updateCtx, updates, config.Load, 10*time.Second)
	var pauseItem *application.MenuItem
	pause := func() {
		application.InvokeSync(func() {
			applyToggle(srv, func(b []byte) { tray.SetTemplateIcon(b) }, func(l string) { pauseItem.SetLabel(l) })
		})
	}
	actions := []func(*application.Context){
		func(*application.Context) { pause() },
		func(*application.Context) { openSettings(app, srv, updates, "") },
		func(*application.Context) { openSettings(app, srv, updates, "about") },
		func(*application.Context) { quit(srv, app.Quit) },
	}
	for i, it := range menuItems {
		item := menu.Add(it.Label).SetAccelerator(it.Accelerator).OnClick(actions[i])
		if i == 0 {
			pauseItem = item
		}
		if i == len(menuItems)-2 {
			menu.AddSeparator()
		}
	}
	tray.SetMenu(menu)
	setupSessions(app, tray, srv, pause, func() { openSettings(app, srv, updates, "") })
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		startPanel(app, srv, cfg)
		startQuestion(app, srv)
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

// aboutText is the app description.
func aboutText() string {
	return "Version " + Version + "\nTray app and hooks for Claude Code.\nnudged is a service by donePM."
}

// updateLabel is the label of the update item for state i.
func updateLabel(i updateInfo) string {
	if i.Checking {
		return "Updating to " + i.Latest + "…"
	}
	return "Update to " + i.Latest + "…"
}

// installUpdate installs the newest release, starts the new app and quits. On error the settings view and the log show it.
func installUpdate(app *application.App, srv *daemon.Server, u *updater) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if !u.apply(ctx) {
		if e := u.state().Error; e != "" {
			fmt.Fprintln(logw, "nudged: update:", e)
		}
		return
	}
	if err := relaunch(); err != nil {
		u.set(func(i *updateInfo) { i.Error = "installed, but restart failed: " + err.Error() })
		return
	}
	application.InvokeSync(func() { quit(srv, app.Quit) })
}
