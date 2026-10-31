package app

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/rokde/nudged/internal/config"
	"github.com/rokde/nudged/internal/proto"
)

// shown reports whether the config shows panels of kind. A kind with no config entry shows.
func shown(kind string, cfg config.Config) bool {
	switch kind {
	case proto.Permission:
		return cfg.Events.Permission.Show
	case proto.Question:
		return cfg.Events.Question.Show
	case proto.Done:
		return cfg.Events.Done.Show
	case proto.Waits:
		return cfg.Events.Waits.Show
	}
	return true
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
