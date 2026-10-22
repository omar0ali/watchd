// Package ipc implements the control protocol between the CLI and the daemon:
// newline-delimited JSON over a Unix socket that only the owning user can
// reach.
package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/omar0ali/watchd/internal/api"
)

const (
	maxRequestSize = 1 << 20 // 1 MiB; requests carry the CLI env, nothing large
	requestTimeout = 10 * time.Second
)

// Writer sends messages back to the client. It is safe for concurrent use.
type Writer struct {
	mu  sync.Mutex
	enc *json.Encoder
}

func (w *Writer) Send(m api.Message) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.enc.Encode(m)
}

// Handler serves one request. ctx is cancelled when the client disconnects.
// Returning an error sends it to the client.
type Handler func(ctx context.Context, req *api.Request, w *Writer) error

type Server struct {
	ln  net.Listener
	log *log.Logger
}

// Listen creates the socket with mode 0600. Any stale socket at path is
// removed, so the caller must already hold the daemon lock.
func Listen(path string, logger *log.Logger) (*Server, error) {
	_ = os.Remove(path)
	old := syscall.Umask(0o177)
	ln, err := net.Listen("unix", path)
	syscall.Umask(old)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return &Server{ln: ln, log: logger}, nil
}

// Serve accepts connections until Close is called.
func (s *Server) Serve(h Handler) error {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			s.log.Println("accept:", err)
			continue
		}
		go s.serveConn(conn, h)
	}
}

func (s *Server) Close() error { return s.ln.Close() }

func (s *Server) serveConn(conn net.Conn, h Handler) {
	defer conn.Close()
	defer func() {
		if r := recover(); r != nil {
			s.log.Printf("panic while handling request: %v", r)
		}
	}()

	if err := checkPeer(conn); err != nil {
		s.log.Println("rejected connection:", err)
		return
	}

	w := &Writer{enc: json.NewEncoder(conn)}

	_ = conn.SetReadDeadline(time.Now().Add(requestTimeout))
	r := bufio.NewReader(io.LimitReader(conn, maxRequestSize))
	line, err := r.ReadBytes('\n')
	if err != nil {
		_ = w.Send(api.Message{Error: "bad request: " + err.Error(), Done: true})
		return
	}
	var req api.Request
	if err := json.Unmarshal(line, &req); err != nil {
		_ = w.Send(api.Message{Error: "bad request: " + err.Error(), Done: true})
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	// The client never sends more than one line, so any read returning
	// means it went away. That lets long-running handlers (logs -f) stop.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_, _ = io.Copy(io.Discard, conn)
		cancel()
	}()

	if err := h(ctx, &req, w); err != nil {
		_ = w.Send(api.Message{Error: err.Error(), Done: true})
		return
	}
	_ = w.Send(api.Message{Done: true})
}

// ErrNoDaemon is returned by Dial when nothing is listening on the socket.
var ErrNoDaemon = errors.New("daemon is not running (start it with: watchd on)")

// Do sends req and calls fn for every message until the response is done.
func Do(socket string, req *api.Request, fn func(api.Message) error) error {
	return DoContext(context.Background(), socket, req, fn)
}

// DoContext is Do, but closes the connection when ctx is cancelled, which
// ends a streaming request such as logs -f. It then returns ctx.Err().
func DoContext(ctx context.Context, socket string, req *api.Request, fn func(api.Message) error) error {
	conn, err := net.Dial("unix", socket)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
			return ErrNoDaemon
		}
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return err
	}

	dec := json.NewDecoder(bufio.NewReader(conn))
	for {
		var m api.Message
		if err := dec.Decode(&m); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(err, io.EOF) {
				return fmt.Errorf("daemon closed the connection unexpectedly")
			}
			return err
		}
		if m.Error != "" {
			return errors.New(m.Error)
		}
		if m.Done {
			return nil
		}
		if fn != nil {
			if err := fn(m); err != nil {
				return err
			}
		}
	}
}
