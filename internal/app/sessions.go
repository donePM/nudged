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
}

type usagePayload struct {
	Tokens  int64   `json:"tokens"`
	CostUSD float64 `json:"cost_usd"`
}

type sessionsPayload struct {
	Sessions []sessionRow `json:"sessions"`
	Usage    usagePayload `json:"usage"`
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

// buildPayload keeps the order of list. Registry.List already sorts it.
func buildPayload(list []proto.Session, t usage.Totals) sessionsPayload {
	rows := make([]sessionRow, 0, len(list))
	for _, s := range list {
		rows = append(rows, sessionRow{s.Project, s.Path, s.Status, s.Term, s.Model, s.Effort, s.ContextPct, s.ID, s.LastActivity})
	}
	return sessionsPayload{rows, usagePayload{
		Tokens:  t.InputTokens + t.OutputTokens + t.CacheCreationTokens + t.CacheReadTokens,
		CostUSD: t.CostUSD,
	}}
}

// focusSession takes the TTY from the registry, never from the webview.
func focusSession(reg *daemon.Registry, id string, jump func(tty, term string)) {
	for _, s := range reg.List() {
		if s.ID == id {
			jump(s.TTY, s.Term)
			return
		}
	}
}

// setupSessions makes the session list window, attaches it to the tray and keeps it and the badge up to date.
func setupSessions(app *application.App, tray *application.SystemTray, srv *daemon.Server) {
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:            "sessions",
		Title:           "Sessions",
		URL:             "/#/Sessions",
		Width:           640,
		Height:          360,
		Hidden:          true,
		Frameless:       true,
		DisableResize:   true,
		HideOnEscape:    true,
		HideOnFocusLost: true,
	})
	tray.AttachWindow(win).WindowOffset(5)

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
			app.Event.Emit("sessions:update", buildPayload(list, totals))
		}
	}()
}
