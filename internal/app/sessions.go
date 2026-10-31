package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/donepm/nudged/internal/daemon"
	"github.com/donepm/nudged/internal/focus"
	"github.com/donepm/nudged/internal/proto"
	"github.com/donepm/nudged/internal/usage"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type sessionRow struct {
	Project      string    `json:"project"`
	Path         string    `json:"path"`
	Status       string    `json:"status"`
	Term         string    `json:"term"`
	Model        string    `json:"model"`
	Effort       string    `json:"effort"`
	ContextPct   int       `json:"context_pct"`
	SessionID    string    `json:"session_id"`
	LastActivity time.Time `json:"last_activity"`
	// PendingKind and PendingText describe the open permission or question of the session, if there is one.
	PendingKind string `json:"pending_kind,omitempty"`
	PendingText string `json:"pending_text,omitempty"`
}

type usagePayload struct {
	Tokens  int64   `json:"tokens"`
	CostUSD float64 `json:"cost_usd"`
}

type sessionsPayload struct {
	Sessions []sessionRow `json:"sessions"`
	Usage    usagePayload `json:"usage"`
	Paused   bool         `json:"paused"`
}

func waitingCount(list []proto.Session) int {
	n := 0
	for _, s := range list {
		if s.Status == "waiting" {
			n++
		}
	}
	return n
}

func badgeLabel(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// pendingText is the short line the session list shows for an open request: "Allow Bash?" or the question.
func pendingText(req proto.PendingRequest) string {
	g := genericPanel(req)
	switch {
	case req.Kind == proto.Permission && g.Tool != "":
		return "Allow " + g.Tool + "?"
	case req.Kind == proto.Permission:
		return "Needs permission"
	case g.Question != "":
		return g.Question
	}
	return "Asks a question"
}

// buildPayload keeps the order of list. Registry.List already sorts it.
// Each session gets the first open permission or question of pending with its session id.
func buildPayload(list []proto.Session, pending []proto.PendingRequest, t usage.Totals, paused bool) sessionsPayload {
	rows := make([]sessionRow, 0, len(list))
	for _, s := range list {
		r := sessionRow{
			Project: s.Project, Path: s.Path, Status: s.Status, Term: s.Term, Model: s.Model, Effort: s.Effort,
			ContextPct: s.ContextPct, SessionID: s.ID, LastActivity: s.LastActivity,
		}
		for _, p := range pending {
			if p.SessionID == s.ID && (p.Kind == proto.Permission || p.Kind == proto.Question) {
				r.PendingKind, r.PendingText = p.Kind, pendingText(p)
				break
			}
		}
		rows = append(rows, r)
	}
	return sessionsPayload{rows, usagePayload{
		Tokens:  t.InputTokens + t.OutputTokens + t.CacheCreationTokens + t.CacheReadTokens,
		CostUSD: t.CostUSD,
	}, paused}
}

// The session list is a popover under the tray icon. The page reports its height, like the panel does.
const sessionsWidth, sessionsMinHeight, sessionsMaxHeight = 380, 120, 640

func init() { application.RegisterEvent[panelSize]("sessions:size") }

// focusSession takes the TTY from the registry, never from the webview.
func focusSession(reg *daemon.Registry, id string, jump func(tty, term string)) {
	for _, s := range reg.List() {
		if s.ID == id {
			jump(s.TTY, s.Term)
			return
		}
	}
}

// sessionsOptions are the options of the session list window.
func sessionsOptions() application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Name:            "sessions",
		Title:           "Sessions",
		URL:             "/#/Sessions",
		Width:           sessionsWidth,
		Height:          sessionsMinHeight,
		Hidden:          true,
		Frameless:       true,
		DisableResize:   true,
		HideOnEscape:    true,
		HideOnFocusLost: true,
		BackgroundType:  application.BackgroundTypeTranslucent,
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTranslucent, CornerRadius: 12,
		},
	}
}

// setupSessions makes the session list window, attaches it to the tray and keeps it and the badge up to date.
// onPause and onSettings run the Pause and Settings items of the tray menu.
func setupSessions(app *application.App, tray *application.SystemTray, srv *daemon.Server, onPause, onSettings func()) {
	win := app.Window.NewWithOptions(sessionsOptions())
	tray.AttachWindow(win).WindowOffset(5)
	height := sessionsMinHeight // only the main thread uses height
	app.Event.On("sessions:size", func(e *application.CustomEvent) {
		z, ok := e.Data.(panelSize)
		if e.Sender != "sessions" || !ok {
			return
		}
		application.InvokeSync(func() {
			h := min(max(z.Height, sessionsMinHeight), sessionsMaxHeight)
			if h == height {
				return
			}
			height = h
			win.SetSize(sessionsWidth, h)
			if win.IsVisible() { // a resize keeps the bottom left corner, so dock the window under the icon again
				_ = tray.PositionWindow(win, 5)
			}
		})
	})
	app.Event.On("sessions:pause", func(*application.CustomEvent) { onPause() })
	app.Event.On("sessions:settings", func(*application.CustomEvent) {
		application.InvokeSync(func() { win.Hide() })
		onSettings()
	})

	ready := make(chan struct{}, 1)
	app.Event.On("sessions:ready", func(*application.CustomEvent) {
		select {
		case ready <- struct{}{}:
		default:
		}
	})
	app.Event.On("sessions:focus", func(e *application.CustomEvent) {
		if id, ok := e.Data.(string); ok {
			focusSession(srv.Registry, id, focus.Jump)
		}
	})

	home, _ := os.UserHomeDir()
	root := filepath.Join(home, ".claude", "projects")
	// shortcut: polling, upgrade when the daemon publishes changes on a channel
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		var totals usage.Totals
		var scanned time.Time
		badge := ""
		for {
			isReady := false
			select {
			case <-tick.C:
			case <-ready:
				isReady = true
			}
			// Usage scans all logs, so scan only for a visible or new view, and at most once each 30 s.
			if time.Since(scanned) >= 30*time.Second && (isReady || win.IsVisible()) {
				scanned = time.Now()
				if t, err := usage.Today(root, scanned); err != nil {
					fmt.Fprintln(logw, "nudged: usage:", err)
				} else {
					totals = t
				}
			}
			list := srv.Registry.List()
			if l := badgeLabel(waitingCount(list)); l != badge {
				badge = l
				tray.SetLabel(l)
			}
			app.Event.Emit("sessions:update", buildPayload(list, srv.Pending(), totals, srv.Paused()))
		}
	}()
}
