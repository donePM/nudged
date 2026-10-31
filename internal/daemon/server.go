package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/rokde/nudged/internal/proto"
)

type Server struct {
	Registry *Registry
	ln       net.Listener
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
	return &Server{Registry: reg, ln: ln}, nil
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

// Close stops the server. The listener removes the socket file.
func (s *Server) Close() error {
	return s.ln.Close()
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	// A client that sends no line must not hold the goroutine open.
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	var req proto.Request
	if proto.ReadLine(bufio.NewReader(conn), &req) != nil {
		return
	}
	proto.WriteLine(conn, s.apply(req))
}

func (s *Server) apply(req proto.Request) proto.Reply {
	switch req.Kind {
	case proto.Hook:
		s.Registry.Hook(req.Event, req.Payload, req.Term, req.TTY)
	case proto.Statusline:
		var info proto.StatuslineInfo
		if json.Unmarshal(req.Payload, &info) == nil {
			s.Registry.Statusline(info)
		}
	case proto.Status:
		out, err := json.Marshal(s.Registry.List())
		if err != nil {
			return proto.Reply{Error: err.Error()}
		}
		return proto.Reply{OK: true, Output: out}
	default:
		return proto.Reply{Error: "unknown kind"}
	}
	return proto.Reply{OK: true}
}
