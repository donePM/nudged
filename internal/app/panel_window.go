package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/donepm/nudged/internal/config"
	"github.com/donepm/nudged/internal/daemon"
	"github.com/donepm/nudged/internal/focus"
	"github.com/donepm/nudged/internal/proto"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const panelName = "panel"

type panelShow struct {
	Request proto.PendingRequest `json:"request"`
	Others  int                  `json:"others"`
	Mode    string               `json:"mode"`     // "answer" or "notify"
	Corner  string               `json:"position"` // config.Positions, where the card sits
	// Permission is set when Request.Kind is proto.Permission.
	Permission *permissionData `json:"permission,omitempty"`
	// Notice is set when Request.Kind is a notice kind.
	Notice *noticeData `json:"notice,omitempty"`
	// Generic is set when the Generic view shows Request.
	Generic *genericData `json:"generic,omitempty"`
	// HideAt is when the panel dismisses the notice by itself. HideMs is the full hide time. Both are zero when the notice stays.
	HideAt *time.Time `json:"hide_at,omitempty"`
	HideMs int64      `json:"hide_ms,omitempty"`
}

// panelSize is the height of the card in the panel page.
type panelSize struct {
	Height int `json:"height"`
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
	application.RegisterEvent[panelSize]("panel:size")
}

// The panel is a small card in a corner of the work area (top right by default), like a macOS notification.
// The page reports the card height, and the panel grows or shrinks to it within the limits.
const panelWidth, panelHeight, panelMargin = 340, 72, 10
const panelMinHeight, panelMaxHeight = 40, 560

// panelOptions returns the options of the panel window on screen s (nil is the main screen).
// The non-activating NSPanel does not take key focus from the terminal.
func panelOptions(view string, s *application.Screen) application.WebviewWindowOptions {
	o := application.WebviewWindowOptions{
		Name: panelName, URL: "/#/" + view, Width: panelWidth, Height: panelHeight,
		Frameless: true, AlwaysOnTop: true, DisableResize: true, Hidden: true,
		BackgroundType: application.BackgroundTypeTranslucent,
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTranslucent, CornerRadius: 14,
			WindowClass: application.MacWindowClassPanel,
			PanelPreferences: application.MacPanelPreferences{
				NonActivating: true, FloatingPanel: true, BecomesKeyOnlyIfNeeded: true,
			},
		},
	}
	if s != nil {
		o.InitialPosition, o.Screen = application.WindowXY, s
		o.X, o.Y = panelXY(s, "top-right", panelHeight)
	}
	return o
}

// panelXY is the place of a panel with height h in corner pos of the work area of s. y counts from the top.
// An unknown pos is the top right corner.
func panelXY(s *application.Screen, pos string, h int) (x, y int) {
	x, y = max(0, s.WorkArea.Width-panelWidth-panelMargin), panelMargin
	if strings.HasSuffix(pos, "-left") {
		x = panelMargin
	}
	if strings.HasPrefix(pos, "bottom-") {
		y = max(0, s.WorkArea.Height-h-panelMargin)
	}
	return x, y
}

// panelFit clamps the card height h to the panel limits.
func panelFit(h int) int { return min(max(h, panelMinHeight), panelMaxHeight) }

// startPanel creates the hidden panel window and shows the top request of each snapshot. Call it after the app started.
func startPanel(app *application.App, srv *daemon.Server, cfg config.Config) {
	view := viewFor("")
	w := app.Window.NewWithOptions(panelOptions(view, app.Screen.GetPrimary()))
	height := panelHeight // only the main thread uses height
	var cur *panelShow    // only the main thread uses cur and view
	// place sizes the panel and puts it in the corner of the config. A resize keeps the bottom left corner, so place runs after each one.
	place := func() {
		w.SetSize(panelWidth, height)
		if s := app.Screen.GetPrimary(); s != nil {
			pos := ""
			if cur != nil {
				pos = cur.Corner
			}
			w.SetRelativePosition(panelXY(s, pos, height))
		}
	}
	// The notice can already be gone, for example after the next hook of its session.
	dismiss := func(id string) { srv.Dismiss(id) }
	app.Event.On("panel:ready", func(e *application.CustomEvent) {
		application.InvokeSync(func() {
			if cur != nil && e.Sender == panelName {
				w.EmitEvent("panel:show", *cur)
			}
		})
	})
	app.Event.On("panel:size", func(e *application.CustomEvent) {
		z, ok := e.Data.(panelSize)
		if e.Sender != panelName || !ok {
			return
		}
		application.InvokeSync(func() {
			if h := panelFit(z.Height); h != height {
				height = h
				place()
			}
		})
	})
	app.Event.On("panel:answer", func(e *application.CustomEvent) {
		a, ok := e.Data.(panelAnswer)
		if e.Sender != panelName || !ok {
			return
		}
		if err := answerPanel(srv.Pending(), a, srv.Answer); err != nil {
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
	go runPanel(srv.Changes(), panelConfigs, cfg, dismiss, func(show *panelShow, timeoutMinutes int) {
		if show != nil && show.Request.Kind == proto.Permission {
			p := permissionPanel(show.Request, srv.Registry.List(), timeoutMinutes)
			show.Permission = &p
		}
		if show != nil && notice(show.Request.Kind) {
			n := noticePanel(show.Request, srv.Registry.List())
			show.Notice = &n
		}
		application.InvokeSync(func() {
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
			place()
		})
	})
}

// answerPanel builds the reply from the pending request a.ID, never from the webview. It calls answer only for a valid reply.
func answerPanel(pending []proto.PendingRequest, a panelAnswer, answer func(string, json.RawMessage) error) error {
	for _, p := range pending {
		if p.ID != a.ID {
			continue
		}
		out, err := decision(a.Behavior)
		if p.Kind == proto.Permission {
			out, err = permissionDecision(a.Behavior, a.Suggestion, p.Payload)
		}
		if err != nil {
			return err
		}
		return answer(a.ID, out)
	}
	return errors.New("no pending request " + strconv.Quote(a.ID))
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
// For each snapshot and each config, it calls show with the top request (nil hides the panel) and the current timeout.
// When a notice with a HideSeconds above 0 becomes the top request, it calls dismiss with the notice id after that time, once for each id.
// The show of such a notice has the hide time and the hide deadline.
// A new config stops the running timers, and the top notice gets a new timer with the new time.
func runPanel(changes <-chan []proto.PendingRequest, configs <-chan config.Config, cfg config.Config, dismiss func(string), show func(*panelShow, int)) {
	var list []proto.PendingRequest
	var seen map[string]bool
	timed := map[string]*time.Timer{}
	hideAt := map[string]time.Time{}
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
			for id, t := range timed {
				t.Stop()
				delete(timed, id)
				delete(hideAt, id)
			}
		}
		var s *panelShow
		if req, others, ok := top(list, cfg); ok {
			s = &panelShow{Request: req, Others: others, Mode: cfg.Mode, Corner: cfg.Position}
			if panelView(s) == "Generic" {
				g := genericPanel(req)
				s.Generic = &g
			}
			ev, _ := event(req.Kind, cfg)
			d := time.Duration(ev.HideSeconds) * hideUnit
			if notice(req.Kind) && timed[req.ID] == nil && d > 0 {
				timed[req.ID] = time.AfterFunc(d, func() { dismiss(req.ID) })
				hideAt[req.ID] = time.Now().Add(d)
			}
			if at, ok := hideAt[req.ID]; ok {
				s.HideAt, s.HideMs = &at, d.Milliseconds()
			}
		}
		for id := range timed {
			if !slices.ContainsFunc(list, func(p proto.PendingRequest) bool { return p.ID == id }) {
				delete(timed, id)
				delete(hideAt, id)
			}
		}
		show(s, cfg.TimeoutMinutes)
	}
}
