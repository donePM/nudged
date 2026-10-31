package app

import (
	"fmt"
	"time"

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
	// Permission is set when Request.Kind is proto.Permission.
	Permission *permissionData `json:"permission,omitempty"`
}

type panelAnswer struct {
	ID       string `json:"id"`
	Behavior string `json:"behavior"`
	// Suggestion is the permission_suggestions index for behavior "always".
	Suggestion int `json:"suggestion,omitempty"`
}

type panelFocus struct {
	ID string `json:"id"`
}

var jump = focus.Jump

// hideUnit is the unit of config.Event.HideSeconds. Tests make it short.
var hideUnit = time.Second

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
	// The notice can already be gone, for example after the next hook of its session.
	dismiss := func(id string) { srv.Dismiss(id) }
	var cur *panelShow // only the main thread uses cur, held and view
	var held []proto.PendingRequest
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
		var req proto.PendingRequest
		application.InvokeSync(func() {
			for _, p := range held {
				if p.ID == a.ID {
					req = p
				}
			}
		})
		out, err := decision(a.Behavior)
		if req.Kind == proto.Permission {
			out, err = permissionDecision(a.Behavior, a.Suggestion, req.Payload)
		}
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
		focusRequest(c, f.ID, dismiss)
	})
	go runPanel(srv.Changes(), panelConfigs, cfg, dismiss, func(list []proto.PendingRequest, show *panelShow, timeoutMinutes int) {
		if show != nil && show.Request.Kind == proto.Permission {
			p := permissionPanel(show.Request, srv.Registry.List(), timeoutMinutes)
			show.Permission = &p
		}
		application.InvokeSync(func() {
			held = list
			cur = show
			if show == nil {
				w.Hide()
				return
			}
			if v := panelView(show); v != view {
				view = v
				w.SetURL("/#/" + v) // the new page sends panel:ready
			}
			w.EmitEvent("panel:show", *show)
			w.Show()
		})
	})
}

// focusRequest brings the terminal of the current request cur to the front, if focusTarget allows it. Then it dismisses a notice.
func focusRequest(cur *panelShow, id string, dismiss func(string)) {
	if tty, term, ok := focusTarget(cur, id); ok {
		jump(tty, term)
		if notice(cur.Request.Kind) {
			dismiss(id)
		}
	}
}

// runPanel keeps the last snapshot and the current config until changes closes.
// For each snapshot and each config, it calls show with the snapshot, the top request (nil hides the panel) and the current timeout.
// For each new notice with a HideSeconds above 0, it calls dismiss with the notice id after that time.
func runPanel(changes <-chan []proto.PendingRequest, configs <-chan config.Config, cfg config.Config, dismiss func(string), show func([]proto.PendingRequest, *panelShow, int)) {
	var list []proto.PendingRequest
	var seen map[string]bool
	for {
		select {
		case l, ok := <-changes:
			if !ok {
				return
			}
			list = l
			for _, p := range list {
				if ev, _ := event(p.Kind, cfg); notice(p.Kind) && !seen[p.ID] && ev.HideSeconds > 0 {
					time.AfterFunc(time.Duration(ev.HideSeconds)*hideUnit, func() { dismiss(p.ID) })
				}
			}
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
		var s *panelShow
		if req, others, ok := top(list, cfg); ok {
			s = &panelShow{Request: req, Others: others, Mode: cfg.Mode}
		}
		show(list, s, cfg.TimeoutMinutes)
	}
}
