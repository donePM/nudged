package daemon

import (
	"bufio"
	"cmp"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/rokde/nudged/internal/proto"
)

type Server struct {
	Registry *Registry
	// HoldTimeout ends a held request with no output. Zero means 30 min. Set it before Serve.
	HoldTimeout time.Duration
	// LineDeadline limits the time to read the request line. Zero means 5 s. Set it before Serve.
	LineDeadline time.Duration
	ln           net.Listener
	queue        queue
}

// Listen creates the socket at path. It removes a stale socket file and fails when a daemon listens on it.
func Listen(path string, reg *Registry) (*Server, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if fi, err := os.Lstat(path); err == nil {
		if fi.Mode().Type() != os.ModeSocket {
			return nil, errors.New(path + " exists and is not a socket")
		}
		if c, err := net.Dial("unix", path); err == nil {
			c.Close()
			return nil, errors.New("a daemon already listens on " + path)
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, proto.SocketMode); err != nil {
		ln.Close()
		return nil, err
	}
	return &Server{Registry: reg, ln: ln, queue: queue{changes: make(chan []proto.PendingRequest, 1)}}, nil
}

// Serve accepts connections until Close. It returns nil after Close.
func (s *Server) Serve() error {
	for {
		conn, err := s.ln.Accept()
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		if err != nil {
			return err
		}
		go s.handle(conn)
	}
}

// Close stops the server and ends all held requests with no output. The listener removes the socket file.
func (s *Server) Close() error {
	s.queue.setPaused(true)
	return s.ln.Close()
}

// SetPaused sets the pause flag. Pause ends all held requests, and while paused the server holds no new request.
func (s *Server) SetPaused(paused bool) {
	s.queue.setPaused(paused)
}

// Changes gets a snapshot of the pending requests, oldest first, after each add and each end. An unread snapshot is replaced by the newest one.
func (s *Server) Changes() <-chan []proto.PendingRequest {
	return s.queue.changes
}

// Answer ends the held request id and sends payload verbatim as the hook output. It is the same path as `nudged answer`.
func (s *Server) Answer(id string, payload json.RawMessage) error {
	return s.queue.answer(id, payload)
}

// Paused reports the pause flag.
func (s *Server) Paused() bool {
	s.queue.mu.Lock()
	defer s.queue.mu.Unlock()
	return s.queue.paused
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	// A client that sends no line must not hold the goroutine open.
	conn.SetDeadline(time.Now().Add(cmp.Or(s.LineDeadline, 5*time.Second)))
	r := bufio.NewReader(conn)
	var req proto.Request
	if proto.ReadLine(r, &req) != nil {
		// The write fails at once when the deadline passed or the client left.
		proto.WriteLine(conn, proto.Reply{Error: "bad request line"})
		return
	}
	rep, h := s.apply(req)
	if h != nil {
		rep = s.hold(conn, r, h)
	}
	proto.WriteLine(conn, rep)
}

// hold waits for the one reply of h. A timeout or a closed client connection ends h with no output.
func (s *Server) hold(conn net.Conn, r *bufio.Reader, h *held) proto.Reply {
	conn.SetDeadline(time.Time{})
	t := time.AfterFunc(cmp.Or(s.HoldTimeout, 30*time.Minute), func() { s.queue.end(h, proto.Reply{OK: true}) })
	defer t.Stop()
	// The read stops at EOF, or when handle closes conn.
	go func() {
		io.Copy(io.Discard, r)
		s.queue.end(h, proto.Reply{OK: true})
	}()
	return <-h.done
}

// apply runs one request. It returns a held request when the hook must wait for an answer.
func (s *Server) apply(req proto.Request) (proto.Reply, *held) {
	switch req.Kind {
	case proto.Hook:
		s.Registry.Hook(req.Event, req.Payload, req.Term, req.TTY)
		var p struct {
			SessionID string `json:"session_id"`
			ToolName  string `json:"tool_name"`
		}
		json.Unmarshal(req.Payload, &p)
		// Claude Code can send a Notification while its permission prompt still waits.
		if p.SessionID != "" && req.Event != "Notification" {
			s.queue.endWhere(func(h *held) bool { return h.SessionID == p.SessionID })
		}
		if kind := holdKind(req.Event, p.ToolName); kind != "" {
			h := s.queue.add(proto.PendingRequest{Kind: kind, SessionID: p.SessionID, Event: req.Event, Payload: req.Payload, Term: req.Term, TTY: req.TTY})
			if h != nil {
				return proto.Reply{}, h
			}
		}
	case proto.Statusline:
		var info proto.StatuslineInfo
		if json.Unmarshal(req.Payload, &info) == nil {
			s.Registry.Statusline(info)
		}
	case proto.Status:
		return output(s.Registry.List()), nil
	case proto.Pending:
		return output(s.queue.pending()), nil
	case proto.Answer:
		if err := s.queue.answer(req.ID, req.Payload); err != nil {
			return proto.Reply{Error: err.Error()}, nil
		}
	default:
		return proto.Reply{Error: "unknown kind"}, nil
	}
	return proto.Reply{OK: true}, nil
}

func output(v any) proto.Reply {
	out, err := json.Marshal(v)
	if err != nil {
		return proto.Reply{Error: err.Error()}
	}
	return proto.Reply{OK: true, Output: out}
}
