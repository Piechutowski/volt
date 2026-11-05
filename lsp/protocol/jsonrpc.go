package protocol

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	codeParseError           = -32700
	codeInvalidRequest       = -32600
	codeMethodNotFound       = -32601
	codeInvalidParams        = -32602
	codeInternalError        = -32603
	codeServerNotInitialized = -32002
)

// Handler answers one method with the raw params of its message, returning
// the result of a request or nil (LSP Request Message, Notification Message).
type Handler func(ctx *Context, params json.RawMessage) (any, error)

// Context is how a handler writes to the editor: Notify sends a notification
// and Request sends a request without waiting, its reply read and dropped.
type Context struct {
	Notify  func(method string, params any)
	Request func(method string, params any)
}

// Request makes the handler of a request from a typed function; params that
// do not decode are answered InvalidParams (LSP Response Message).
func Request[P, R any](f func(*Context, *P) (R, error)) Handler {
	return func(ctx *Context, raw json.RawMessage) (any, error) {
		var p P
		if err := paramsDecode(raw, &p); err != nil {
			return nil, err
		}
		return f(ctx, &p)
	}
}

// Notification makes the handler of a notification from a typed function;
// params that do not decode drop the notification (LSP Notification Message).
func Notification[P any](f func(*Context, *P) error) Handler {
	return func(ctx *Context, raw json.RawMessage) (any, error) {
		var p P
		if err := paramsDecode(raw, &p); err != nil {
			return nil, err
		}
		return nil, f(ctx, &p)
	}
}

func paramsDecode(raw json.RawMessage, p any) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, p); err != nil {
		return &responseError{Code: codeInvalidParams, Message: "params do not decode: " + err.Error()}
	}
	return nil
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *responseError) Error() string { return e.Message }

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

type outgoing struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// Serve answers the messages read from r on w, one at a time in arrival
// order, until exit or the end of r, and returns the exit code the LSP Exit
// Notification section gives; an error means the stream itself broke.
func Serve(r io.Reader, w io.Writer, handlers map[string]Handler) (int, error) {
	s := &server{handlers: handlers, out: &writer{w: w}}
	s.ctx = &Context{Notify: s.out.notify, Request: s.out.request}
	rd := bufio.NewReader(r)
	for {
		body, err := messageRead(rd)
		if err == io.EOF {
			return s.exitCode(), nil
		}
		if err != nil {
			return 1, err
		}
		exit, err := s.handle(body)
		if err != nil {
			return 1, err
		}
		if exit {
			return s.exitCode(), nil
		}
	}
}

func messageRead(rd *bufio.Reader) ([]byte, error) {
	length := -1
	for first := true; ; first = false {
		line, err := rd.ReadString('\n')
		if err == io.EOF && first && line == "" {
			return nil, io.EOF
		}
		if err != nil {
			return nil, fmt.Errorf("reading a message header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("malformed header line %q", line)
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil || n < 0 {
				return nil, fmt.Errorf("invalid Content-Length %q", strings.TrimSpace(value))
			}
			length = n
		}
	}
	if length < 0 {
		return nil, errors.New("a message header without Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(rd, body); err != nil {
		return nil, fmt.Errorf("reading a message body: %w", err)
	}
	return body, nil
}

type server struct {
	handlers    map[string]Handler
	ctx         *Context
	out         *writer
	initialized bool
	shut        bool
}

func (s *server) exitCode() int {
	if s.shut {
		return 0
	}
	return 1
}

func (s *server) handle(body []byte) (exit bool, err error) {
	var m message
	if err := json.Unmarshal(body, &m); err != nil {
		code := codeInvalidRequest
		var syntax *json.SyntaxError
		if errors.As(err, &syntax) {
			code = codeParseError
		}
		return false, s.out.reply(nil, nil, &responseError{Code: code, Message: err.Error()})
	}
	request := m.ID != nil
	switch {
	case m.Method == "" && request && (m.Result != nil || m.Error != nil):
		return false, nil
	case request && !idValid(m.ID):
		return false, s.out.reply(nil, nil, &responseError{Code: codeInvalidRequest, Message: "a request id is a number or a string"})
	case m.Method == "" || m.JSONRPC != "2.0":
		if !request {
			return false, nil
		}
		return false, s.out.reply(m.ID, nil, &responseError{Code: codeInvalidRequest, Message: `a request carries "jsonrpc": "2.0" and a method`})
	case m.Method == "exit":
		return true, nil
	case !request:
		s.notification(&m)
		return false, nil
	}
	result, err := s.request(&m)
	return false, s.out.reply(m.ID, result, err)
}

func idValid(id json.RawMessage) bool {
	c := id[0]
	return c == '"' || c == '-' || ('0' <= c && c <= '9')
}

func (s *server) notification(m *message) {
	if !s.initialized || s.shut {
		return
	}
	if h, ok := s.handlers[m.Method]; ok {
		h(s.ctx, m.Params)
	}
}

func (s *server) request(m *message) (any, error) {
	switch {
	case s.shut:
		return nil, &responseError{Code: codeInvalidRequest, Message: "the server is shut down"}
	case !s.initialized && m.Method != "initialize":
		return nil, &responseError{Code: codeServerNotInitialized, Message: "the server is not initialized"}
	case m.Method == "initialize" && s.initialized:
		return nil, &responseError{Code: codeInvalidRequest, Message: "initialize was already sent"}
	case m.Method == "shutdown":
		s.shut = true
		return nil, nil
	}
	h, ok := s.handlers[m.Method]
	if !ok {
		return nil, &responseError{Code: codeMethodNotFound, Message: "no method " + strconv.Quote(m.Method)}
	}
	result, err := h(s.ctx, m.Params)
	if err == nil && m.Method == "initialize" {
		s.initialized = true
	}
	return result, err
}

type writer struct {
	mu sync.Mutex
	w  io.Writer
	id atomic.Int64
}

func (o *writer) write(v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	frame := make([]byte, 0, len(body)+24)
	frame = append(frame, "Content-Length: "...)
	frame = strconv.AppendInt(frame, int64(len(body)), 10)
	frame = append(frame, "\r\n\r\n"...)
	frame = append(frame, body...)
	o.mu.Lock()
	defer o.mu.Unlock()
	_, err = o.w.Write(frame)
	return err
}

func (o *writer) reply(id json.RawMessage, result any, err error) error {
	r := response{JSONRPC: "2.0", ID: id}
	if err == nil {
		r.Result, err = json.Marshal(result)
	}
	if err != nil {
		r.Result = nil
		if !errors.As(err, &r.Error) {
			r.Error = &responseError{Code: codeInternalError, Message: err.Error()}
		}
	}
	return o.write(r)
}

func (o *writer) notify(method string, params any) {
	o.write(outgoing{JSONRPC: "2.0", Method: method, Params: params})
}

func (o *writer) request(method string, params any) {
	o.write(outgoing{JSONRPC: "2.0", ID: o.id.Add(1), Method: method, Params: params})
}
