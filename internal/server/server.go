package server

import (
	"errors"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"redis-clone/internal/resp"
	"redis-clone/internal/store"
)

// Server owns a Store (shared across every connection's goroutine)
// plus the path where it persists to disk. ln is kept so Close() can
// shut down the listener from outside the Accept loop — that's what
// makes graceful shutdown possible.
type Server struct {
	addr         string
	db           *store.Store
	snapshotPath string
	ln           net.Listener
}

func New(addr, snapshotPath string) *Server {
	return &Server{addr: addr, db: store.New(), snapshotPath: snapshotPath}
}

// LoadSnapshot restores the store from disk. Call this once, before
// ListenAndServe, so data is in place before any client can connect.
func (s *Server) LoadSnapshot() error {
	return s.db.LoadFromFile(s.snapshotPath)
}

// SaveSnapshot writes the store to disk right now.
func (s *Server) SaveSnapshot() error {
	return s.db.SaveToFile(s.snapshotPath)
}

// StartPeriodicSnapshot begins saving on a fixed interval in the
// background — the backstop against a hard kill (SIGKILL) that
// signal handling can never catch. Fire-and-forget: errors are
// logged, not returned, since there's no caller left to hand them to
// once this goroutine is running on its own.
func (s *Server) StartPeriodicSnapshot(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if err := s.SaveSnapshot(); err != nil {
				log.Printf("periodic snapshot failed: %v", err)
			}
		}
	}()
}

// Close shuts down the listener, which causes the Accept() loop in
// ListenAndServe to unblock and return nil (see the net.ErrClosed
// check below) — that's the mechanism graceful shutdown uses to let
// main() get control back and save one last snapshot.
func (s *Server) Close() error {
	if s.ln == nil {
		return nil
	}
	return s.ln.Close()
}

func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.ln = ln
	defer ln.Close()

	log.Printf("listening on %s", s.addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			log.Printf("accept error: %v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

// handleConnection now speaks RESP instead of raw lines: read one
// command (an Array of BulkStrings), dispatch it, write one
// response, repeat.
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	remote := conn.RemoteAddr()
	log.Printf("client connected: %s", remote)
	defer log.Printf("client disconnected: %s", remote)

	reader := resp.NewReader(conn)
	writer := resp.NewWriter(conn)

	for {
		value, err := reader.Read()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("read error for %s: %v", remote, err)
			}
			return
		}

		args, err := parseCommand(value)
		if err != nil {
			_ = writer.Write(resp.ErrorValue("ERR " + err.Error()))
			continue
		}

		response := s.dispatch(args)
		if err := writer.Write(response); err != nil {
			log.Printf("write error for %s: %v", remote, err)
			return
		}
	}
}

// parseCommand validates that an incoming Value is shaped the way a
// real command must be: a non-empty Array of BulkStrings. Anything
// else (a bare integer, a malformed array, etc.) is a protocol
// error, not a command we can dispatch.
func parseCommand(v resp.Value) ([]string, error) {
	if v.Type != resp.Array || len(v.Elements) == 0 {
		return nil, errors.New("expected array of bulk strings for command")
	}
	args := make([]string, len(v.Elements))
	for i, elem := range v.Elements {
		if elem.Type != resp.BulkString {
			return nil, errors.New("command elements must be bulk strings")
		}
		args[i] = elem.Bulk
	}
	return args, nil
}

// dispatch is now a method on Server so command handlers can reach
// s.db. It's still a plain switch — worth revisiting as a
// map[string]func(...) once this grows past ~8 commands, but a
// switch is more readable than that infrastructure for now.
func (s *Server) dispatch(args []string) resp.Value {
	name := strings.ToUpper(args[0])
	switch name {
	case "PING":
		if len(args) > 1 {
			return resp.BulkStringValue(args[1])
		}
		return resp.SimpleStringValue("PONG")
	case "ECHO":
		if len(args) != 2 {
			return resp.ErrorValue("ERR wrong number of arguments for 'echo' command")
		}
		return resp.BulkStringValue(args[1])
	case "SET":
		if len(args) != 3 {
			return resp.ErrorValue("ERR wrong number of arguments for 'set' command")
		}
		s.db.Set(args[1], args[2])
		return resp.SimpleStringValue("OK")
	case "GET":
		if len(args) != 2 {
			return resp.ErrorValue("ERR wrong number of arguments for 'get' command")
		}
		value, ok := s.db.Get(args[1])
		if !ok {
			return resp.NullBulkString()
		}
		return resp.BulkStringValue(value)
	case "DEL":
		if len(args) < 2 {
			return resp.ErrorValue("ERR wrong number of arguments for 'del' command")
		}
		count := s.db.Del(args[1:]...)
		return resp.IntegerValue(int64(count))
	case "EXISTS":
		if len(args) < 2 {
			return resp.ErrorValue("ERR wrong number of arguments for 'exists' command")
		}
		count := s.db.Exists(args[1:]...)
		return resp.IntegerValue(int64(count))
	case "EXPIRE":
		if len(args) != 3 {
			return resp.ErrorValue("ERR wrong number of arguments for 'expire' command")
		}
		seconds, err := strconv.Atoi(args[2])
		if err != nil {
			return resp.ErrorValue("ERR value is not an integer or out of range")
		}
		ok := s.db.Expire(args[1], time.Duration(seconds)*time.Second)
		if !ok {
			return resp.IntegerValue(0)
		}
		return resp.IntegerValue(1)
	case "TTL":
		if len(args) != 2 {
			return resp.ErrorValue("ERR wrong number of arguments for 'ttl' command")
		}
		remaining, exists, hasExpiry := s.db.TTL(args[1])
		if !exists {
			return resp.IntegerValue(-2) // real Redis convention: key doesn't exist
		}
		if !hasExpiry {
			return resp.IntegerValue(-1) // real Redis convention: exists, but permanent
		}
		seconds := int64(remaining.Seconds())
		if seconds < 0 {
			seconds = 0 // guards a race where the key expires between TTL's check and this line
		}
		return resp.IntegerValue(seconds)
	case "PERSIST":
		if len(args) != 2 {
			return resp.ErrorValue("ERR wrong number of arguments for 'persist' command")
		}
		ok := s.db.Persist(args[1])
		if !ok {
			return resp.IntegerValue(0)
		}
		return resp.IntegerValue(1)
	default:
		return resp.ErrorValue("ERR unknown command '" + name + "'")
	}
}
