// Package app is the tray app. It runs the daemon in the same process.
package app

import (
	_ "embed"
	"fmt"
	"os"
	"time"

	"github.com/rokde/nudged/internal/config"
	"github.com/rokde/nudged/internal/daemon"
	"github.com/rokde/nudged/internal/proto"
	"github.com/wailsapp/wails/v3/pkg/application"
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

// toggle flips the pause flag. It returns the new tray icon and the new label of the Pause item.
func toggle(srv *daemon.Server) ([]byte, string) {
	paused := !srv.Paused()
	srv.SetPaused(paused)
	if paused {
		return pausedIcon, "Resume"
	}
	return runningIcon, menuItems[0].Label
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
		OnShutdown: func() { srv.Close() },
	})
	tray := app.SystemTray.New()
	tray.SetTemplateIcon(runningIcon)
	menu := app.NewMenu()
	actions := []func(*application.Context){
		func(ctx *application.Context) {
			icon, label := toggle(srv)
			tray.SetTemplateIcon(icon)
			ctx.ClickedMenuItem().SetLabel(label)
		},
		func(*application.Context) { openSettings() },
		func(*application.Context) { quit(srv, app.Quit) },
	}
	for i, it := range menuItems {
		menu.Add(it.Label).SetAccelerator(it.Accelerator).OnClick(actions[i])
	}
	tray.SetMenu(menu)
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
