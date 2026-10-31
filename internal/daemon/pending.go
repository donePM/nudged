package daemon

import (
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/donepm/nudged/internal/proto"
)

// holdKind returns the pending kind of a hook event, or "" when the daemon replies at once.
func holdKind(event, tool string) string {
	switch {
	case event != "PermissionRequest":
		return ""
	case tool == "AskUserQuestion":
		return proto.Question
	}
	return proto.Permission
}

// noticeKind returns the notice kind of a hook event, or "". A notice does not hold the hook.
// A Stop with running background tasks is not done (docs/spike.md, "Other findings").
func noticeKind(event, notificationType string, backgroundTasks int) string {
	switch {
	case event == "Stop" && backgroundTasks == 0:
		return proto.Done
	case event == "Notification" && notificationType == "idle_prompt":
		return proto.Waits
	}
	return ""
}

type held struct {
	proto.PendingRequest
	done chan proto.Reply // gets exactly one reply, from queue.end
}

// queue holds the pending requests, oldest first. The zero value is ready to use.
type queue struct {
	mu     sync.Mutex
	next   int
	list   []*held
	paused bool
	// changes gets a snapshot after each change. Nil means no snapshots.
	changes chan []proto.PendingRequest
}

// add queues p with a new id. It returns nil while paused.
func (q *queue) add(p proto.PendingRequest) *held {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.paused {
		return nil
	}
	q.next++
	p.ID, p.Started = strconv.Itoa(q.next), time.Now()
	h := &held{PendingRequest: p, done: make(chan proto.Reply, 1)}
	q.list = append(q.list, h)
	q.publishLocked()
	return h
}

// end removes h and sends rep to it. It does nothing when h already left the queue, so each request ends only one time.
func (q *queue) end(h *held, rep proto.Reply) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.endLocked(h, rep)
}

func (q *queue) endLocked(h *held, rep proto.Reply) {
	i := slices.Index(q.list, h)
	if i < 0 {
		return
	}
	q.list = slices.Delete(q.list, i, i+1)
	h.done <- rep
	q.publishLocked()
}

// publishLocked sends a snapshot and never blocks: it replaces an unread snapshot with the new one.
func (q *queue) publishLocked() {
	if q.changes == nil {
		return
	}
	select {
	case <-q.changes:
	default:
	}
	// q.mu makes this the only sender, so the buffer has space now.
	q.changes <- q.pendingLocked()
}

func (q *queue) answer(id string, payload json.RawMessage) error {
	if !json.Valid(payload) {
		return errors.New("payload is not valid JSON")
	}
	return q.endID(id, proto.Reply{OK: true, Output: payload})
}

// endID ends the request id with rep. It returns an error for an unknown id.
func (q *queue) endID(id string, rep proto.Reply) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, h := range q.list {
		if h.ID == id {
			q.endLocked(h, rep)
			return nil
		}
	}
	return errors.New("unknown id " + strconv.Quote(id))
}

// endWhere ends each request for which match is true with no output.
func (q *queue) endWhere(match func(*held) bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, h := range slices.Clone(q.list) {
		if match(h) {
			q.endLocked(h, proto.Reply{OK: true})
		}
	}
}

// setPaused sets the pause flag. Pause ends all held requests.
func (q *queue) setPaused(paused bool) {
	q.mu.Lock()
	q.paused = paused
	q.mu.Unlock()
	if paused {
		q.endWhere(func(*held) bool { return true })
	}
}

func (q *queue) pending() []proto.PendingRequest {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pendingLocked()
}

func (q *queue) pendingLocked() []proto.PendingRequest {
	list := make([]proto.PendingRequest, len(q.list))
	for i, h := range q.list {
		list[i] = h.PendingRequest
	}
	return list
}
