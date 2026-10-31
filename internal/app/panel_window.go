package app

import (
	"fmt"

	"github.com/rokde/nudged/internal/config"
	"github.com/rokde/nudged/internal/daemon"
	"github.com/rokde/nudged/internal/focus"
	"github.com/rokde/nudged/internal/proto"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const panelName = "panel"

type panelShow struct {
	Request proto.PendingRequest `json:"request"`
	Others  int                  `json:"others"`
	Mode    string               `json:"mode"` // "answer" or "notify"
}

type panelAnswer struct {
	ID       string `json:"id"`
	Behavior string `json:"behavior"`
}

type panelFocus struct {
	ID string `json:"id"`
}

var jump = focus.Jump

func init() {
	application.RegisterEvent[panelAnswer]("panel:answer")
	application.RegisterEvent[panelFocus]("panel:focus")
}

const panelWidth, panelHeight, panelMargin = 380, 220, 12

// panelOptions returns the options of the panel window. The non-activating NSPanel does not take key focus from the terminal.
func panelOptions(view string) application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Name: panelName, URL: "/#/" + view, Width: panelWidth, Height: panelHeight,
		Frameless: true, AlwaysOnTop: true, DisableResize: true, Hidden: true,
		Mac: application.MacWindow{
			WindowClass: application.MacWindowClassPanel,
			PanelPreferences: application.MacPanelPreferences{
				NonActivating: true, FloatingPanel: true, BecomesKeyOnlyIfNeeded: true,
			},
		},
	}
}

// startPanel creates the hidden panel window and shows the top request of each snapshot. Call it after the app started.
func startPanel(app *application.App, srv *daemon.Server, cfg config.Config) {
	view := viewFor("")
	w := app.Window.NewWithOptions(panelOptions(view))
	if s := app.Screen.GetPrimary(); s != nil {
		w.SetPosition(s.WorkArea.X+s.WorkArea.Width-panelWidth-panelMargin, s.WorkArea.Y+panelMargin)
	}
	var cur *panelShow // only the main thread uses cur and view
	app.Event.On("panel:ready", func(e *application.CustomEvent) {
		application.InvokeSync(func() {
			if cur != nil && e.Sender == panelName {
				w.EmitEvent("panel:show", *cur)
			}
		})
	})
	app.Event.On("panel:answer", func(e *application.CustomEvent) {
		a, ok := e.Data.(panelAnswer)
		if e.Sender != panelName || !ok {
			return
		}
		out, err := decision(a.Behavior)
		if err == nil {
			err = srv.Answer(a.ID, out)
		}
		if err != nil {
			fmt.Fprintln(logw, "nudged: panel answer:", err)
		}
	})
	app.Event.On("panel:focus", func(e *application.CustomEvent) {
		f, ok := e.Data.(panelFocus)
		if e.Sender != panelName || !ok {
			return
		}
		var c *panelShow
		application.InvokeSync(func() { c = cur })
		focusRequest(c, f.ID)
	})
	go runPanel(srv.Changes(), panelConfigs, cfg, func(show *panelShow) {
		application.InvokeSync(func() {
			cur = show
			if show == nil {
				w.Hide()
				return
			}
			if v := viewFor(show.Request.Kind); v != view {
				view = v
				w.SetURL("/#/" + v) // the new page sends panel:ready
			}
			w.EmitEvent("panel:show", *show)
			w.Show()
		})
	})
}

// focusRequest brings the terminal of the current request cur to the front, if focusTarget allows it.
func focusRequest(cur *panelShow, id string) {
	if tty, term, ok := focusTarget(cur, id); ok {
		jump(tty, term)
	}
}

// runPanel keeps the last snapshot and the current config until changes closes.
// For each snapshot and each config, it calls show with the top request, or with nil to hide the panel.
func runPanel(changes <-chan []proto.PendingRequest, configs <-chan config.Config, cfg config.Config, show func(*panelShow)) {
	var list []proto.PendingRequest
	var seen map[string]bool
	for {
		select {
		case l, ok := <-changes:
			if !ok {
				return
			}
			list = l
			var names []string
			names, seen = sounds(seen, list, cfg)
			for _, name := range names {
				go func() {
					if err := play(name); err != nil {
						fmt.Fprintln(logw, "nudged: sound:", err)
					}
				}()
			}
		case cfg = <-configs:
		}
		if req, others, ok := top(list, cfg); ok {
			show(&panelShow{req, others, cfg.Mode})
		} else {
			show(nil)
		}
	}
}
