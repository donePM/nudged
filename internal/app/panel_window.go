package app

import (
	"fmt"

	"github.com/rokde/nudged/internal/config"
	"github.com/rokde/nudged/internal/daemon"
	"github.com/rokde/nudged/internal/proto"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const panelName = "panel"

type panelShow struct {
	Request proto.PendingRequest `json:"request"`
	Others  int                  `json:"others"`
	// Permission is set when Request.Kind is proto.Permission.
	Permission *permissionData `json:"permission,omitempty"`
}

type panelAnswer struct {
	ID       string `json:"id"`
	Behavior string `json:"behavior"`
	// Suggestion is the permission_suggestions index for behavior "always".
	Suggestion int `json:"suggestion,omitempty"`
}

func init() {
	application.RegisterEvent[panelAnswer]("panel:answer")
}

// startPanel creates the hidden panel window and shows the top request of each snapshot. Call it after the app started.
func startPanel(app *application.App, srv *daemon.Server, cfg config.Config) {
	const width, height, margin = 380, 220, 12
	view := viewFor("")
	w := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: panelName, URL: "/#/" + view, Width: width, Height: height,
		Frameless: true, AlwaysOnTop: true, DisableResize: true, Hidden: true,
	})
	if s := app.Screen.GetPrimary(); s != nil {
		w.SetPosition(s.WorkArea.X+s.WorkArea.Width-width-margin, s.WorkArea.Y+margin)
	}
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
	go func() {
		for list := range srv.Changes() {
			req, others, ok := top(list, cfg)
			var perm *permissionData
			if ok && req.Kind == proto.Permission {
				p := permissionPanel(req, srv.Registry.List(), cfg.TimeoutMinutes)
				perm = &p
			}
			application.InvokeSync(func() {
				held = list
				if !ok {
					cur = nil
					w.Hide()
					return
				}
				cur = &panelShow{req, others, perm}
				if v := viewFor(req.Kind); v != view {
					view = v
					w.SetURL("/#/" + v) // the new page sends panel:ready
				}
				w.EmitEvent("panel:show", *cur)
				w.Show()
			})
		}
	}()
}
