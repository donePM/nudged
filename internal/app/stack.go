package app

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/donepm/nudged/internal/config"
	"github.com/donepm/nudged/internal/proto"
)

// event returns the config of kind. It returns false for a kind with no config entry.
func event(kind string, cfg config.Config) (config.Event, bool) {
	switch kind {
	case proto.Permission:
		return cfg.Events.Permission, true
	case proto.Question:
		return cfg.Events.Question, true
	case proto.Done:
		return cfg.Events.Done, true
	case proto.Waits:
		return cfg.Events.Waits, true
	}
	return config.Event{}, false
}

// shown reports whether the config shows panels of kind. A kind with no config entry shows.
func shown(kind string, cfg config.Config) bool {
	ev, ok := event(kind, cfg)
	return !ok || ev.Show
}

// sounds returns the sound names for the requests of list whose IDs are not in seen, and the IDs of list.
// A request plays a sound only if its kind shows and has the sound on.
func sounds(seen map[string]bool, list []proto.PendingRequest, cfg config.Config) ([]string, map[string]bool) {
	var names []string
	ids := make(map[string]bool, len(list))
	for _, p := range list {
		ids[p.ID] = true
		if ev, ok := event(p.Kind, cfg); ok && !seen[p.ID] && ev.Show && ev.Sound {
			names = append(names, soundNames[p.Kind])
		}
	}
	return names, ids
}

// notice reports whether kind is a notice kind. A notice does not hold its hook.
func notice(kind string) bool { return kind == proto.Done || kind == proto.Waits }

// focusTarget returns the TTY and Term of the current request cur, if cur has the ID id and is a notice or in notify mode.
func focusTarget(cur *panelShow, id string) (tty, term string, ok bool) {
	if cur == nil || cur.Request.ID != id || (cur.Mode != "notify" && !notice(cur.Request.Kind)) {
		return "", "", false
	}
	return cur.Request.TTY, cur.Request.Term, true
}

// panelView returns the view of s. Notify mode uses Generic for each kind except notices: it has no answer controls, and a click sends panel:focus.
func panelView(s *panelShow) string {
	if s.Mode == "notify" && !notice(s.Request.Kind) {
		return "Generic"
	}
	return viewFor(s.Request.Kind)
}

// top returns the oldest shown request of list (oldest first) and the count of the other shown requests.
func top(list []proto.PendingRequest, cfg config.Config) (req proto.PendingRequest, others int, ok bool) {
	for _, p := range list {
		if !shown(p.Kind, cfg) {
			continue
		}
		if ok {
			others++
		} else {
			req, ok = p, true
		}
	}
	return req, others, ok
}

// decision returns the PermissionRequest hook output for behavior "allow" or "deny" (format in docs/spike.md).
func decision(behavior string) (json.RawMessage, error) {
	switch behavior {
	case "allow":
		return json.RawMessage(`{"hookSpecificOutput":{"hookEventName":"PermissionRequest","decision":{"behavior":"allow"}}}`), nil
	case "deny":
		return json.RawMessage(`{"hookSpecificOutput":{"hookEventName":"PermissionRequest","decision":{"behavior":"deny","message":"Denied in nudged panel"}}}`), nil
	}
	return nil, errors.New("unknown behavior " + strconv.Quote(behavior))
}
