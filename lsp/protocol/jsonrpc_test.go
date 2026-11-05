package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func frame(body string) string {
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}

func frames(bodies ...string) string {
	var b strings.Builder
	for _, body := range bodies {
		b.WriteString(frame(body))
	}
	return b.String()
}

const (
	initReq     = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":null,"capabilities":{"workspace":{"didChangeWatchedFiles":{"dynamicRegistration":true}},"textDocument":{"hover":{"contentFormat":["markdown"]}}},"trace":"off"}}`
	initialized = `{"jsonrpc":"2.0","method":"initialized","params":{}}`
	shutdownReq = `{"jsonrpc":"2.0","id":99,"method":"shutdown"}`
	exitNote    = `{"jsonrpc":"2.0","method":"exit"}`
)

type chunkReader struct{ chunks []string }

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(r.chunks) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.chunks[0])
	r.chunks[0] = r.chunks[0][n:]
	if r.chunks[0] == "" {
		r.chunks = r.chunks[1:]
	}
	return n, nil
}

type recorder struct {
	mu    sync.Mutex
	calls []string
}

func (rec *recorder) note(method string) {
	rec.mu.Lock()
	rec.calls = append(rec.calls, method)
	rec.mu.Unlock()
}

func (rec *recorder) handlers() map[string]Handler {
	return map[string]Handler{
		"initialize": Request(func(_ *Context, p *InitializeParams) (InitializeResult, error) {
			rec.note("initialize")
			if !p.Capabilities.Workspace.DidChangeWatchedFiles.DynamicRegistration {
				return InitializeResult{}, errors.New("dynamic registration not read")
			}
			return InitializeResult{Capabilities: ServerCapabilities{HoverProvider: true}}, nil
		}),
		"initialized": Notification(func(ctx *Context, _ *InitializedParams) error {
			rec.note("initialized")
			ctx.Request("client/registerCapability", RegistrationParams{Registrations: []Registration{{ID: "w", Method: "workspace/didChangeWatchedFiles"}}})
			return nil
		}),
		"textDocument/didOpen": Notification(func(_ *Context, p *DidOpenTextDocumentParams) error {
			rec.note("didOpen " + p.TextDocument.URI)
			return nil
		}),
		"textDocument/hover": Request(func(_ *Context, p *HoverParams) (*Hover, error) {
			rec.note("hover")
			if p.Position.Line == 7 {
				return &Hover{Contents: MarkupContent{Kind: MarkupKindMarkdown, Value: "seven"}}, nil
			}
			return nil, nil
		}),
		"textDocument/rename": Request(func(_ *Context, p *RenameParams) (*WorkspaceEdit, error) {
			return nil, errors.New("rename refused: " + p.NewName)
		}),
		"burst": Request(func(ctx *Context, _ *struct{}) (any, error) {
			var wg sync.WaitGroup
			for g := range 8 {
				wg.Go(func() {
					for i := range 50 {
						ctx.Notify(TextDocumentPublishDiagnostics, PublishDiagnosticsParams{URI: fmt.Sprintf("file:///g%d/%d.volt", g, i)})
					}
				})
			}
			wg.Wait()
			return "done", nil
		}),
	}
}

type served struct {
	code  int
	err   error
	raw   []string
	msgs  []map[string]any
	calls []string
	rec   *recorder
}

func serve(t *testing.T, r io.Reader) served {
	t.Helper()
	rec := &recorder{}
	var out bytes.Buffer
	code, err := Serve(r, &out, rec.handlers())
	s := served{code: code, err: err, calls: rec.calls, rec: rec}
	rd := bufio.NewReader(&out)
	for {
		line, err := rd.ReadString('\n')
		if err == io.EOF && line == "" {
			break
		}
		n, ok := strings.CutPrefix(strings.TrimRight(line, "\r\n"), "Content-Length: ")
		length, convErr := strconv.Atoi(n)
		if err != nil || !ok || convErr != nil {
			t.Fatalf("the server wrote a malformed header %q", line)
		}
		if blank, _ := rd.ReadString('\n'); blank != "\r\n" {
			t.Fatalf("the header is not ended by a blank line: %q", blank)
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(rd, body); err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatalf("the server wrote a body that is not JSON: %q", body)
		}
		s.raw = append(s.raw, string(body))
		s.msgs = append(s.msgs, m)
	}
	return s
}

func serveString(t *testing.T, input string) served {
	t.Helper()
	return serve(t, strings.NewReader(input))
}

func errorCode(m map[string]any) int {
	e, ok := m["error"].(map[string]any)
	if !ok {
		return 0
	}
	return int(e["code"].(float64))
}

func expectError(t *testing.T, m map[string]any, id any, code int) {
	t.Helper()
	if m["id"] != id || errorCode(m) != code {
		t.Fatalf("got %v, want an error %d for id %v", m, code, id)
	}
	if _, both := m["result"]; both {
		t.Fatalf("an error response carries a result: %v", m)
	}
}

func session(middle ...string) string {
	return frames(append(append([]string{initReq, initialized}, middle...), shutdownReq, exitNote)...)
}

func TestFramingSeveralMessagesInOneRead(t *testing.T) {
	hover := `{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":"file:///a.volt"},"position":{"line":7,"character":1}}}`
	input := session(hover)
	for name, r := range map[string]io.Reader{
		"one read":          &chunkReader{chunks: []string{input}},
		"one byte per read": &chunkReader{chunks: strings.Split(input, "")},
		"split in headers":  &chunkReader{chunks: []string{input[:10], input[10:300], input[300:]}},
	} {
		t.Run(name, func(t *testing.T) {
			s := serve(t, r)
			if s.err != nil || s.code != 0 {
				t.Fatalf("code %d, err %v", s.code, s.err)
			}
			if len(s.msgs) != 4 {
				t.Fatalf("want 4 messages, got %d: %v", len(s.msgs), s.raw)
			}
			if s.msgs[0]["id"] != 1.0 || s.msgs[0]["result"] == nil {
				t.Fatalf("initialize: %v", s.raw[0])
			}
			if s.msgs[1]["method"] != "client/registerCapability" {
				t.Fatalf("registerCapability: %v", s.raw[1])
			}
			hover := s.msgs[2]["result"].(map[string]any)
			if s.msgs[2]["id"] != 2.0 || hover["contents"].(map[string]any)["value"] != "seven" {
				t.Fatalf("hover: %v", s.raw[2])
			}
			if s.raw[3] != `{"jsonrpc":"2.0","id":99,"result":null}` {
				t.Fatalf("shutdown: %v", s.raw[3])
			}
		})
	}
}

func TestFramingHeaders(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{"workspace":{"didChangeWatchedFiles":{"dynamicRegistration":true}}}}}`
	ok := fmt.Sprintf("content-length: %d\r\nContent-Type: application/vscode-jsonrpc; charset=utf-8\r\n\r\n%s", len(body), body)
	if s := serveString(t, ok); s.err != nil || len(s.msgs) != 1 || s.msgs[0]["result"] == nil {
		t.Fatalf("a lower-case Content-Length with a Content-Type: %v %v", s.err, s.raw)
	}
	for name, input := range map[string]string{
		"missing":        "Content-Type: application/vscode-jsonrpc; charset=utf-8\r\n\r\n{}",
		"not a number":   "Content-Length: ten\r\n\r\n{}",
		"negative":       "Content-Length: -2\r\n\r\n{}",
		"no colon":       "Content-Length 2\r\n\r\n{}",
		"body too short": "Content-Length: 40\r\n\r\n{}",
		"header cut off": "Content-Length: 2\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			s := serveString(t, frame(initReq)+input)
			if s.err == nil || s.code != 1 {
				t.Fatalf("want a stream error and code 1, got code %d err %v", s.code, s.err)
			}
			if len(s.msgs) != 1 {
				t.Fatalf("the message before the bad one is answered, nothing after: %v", s.raw)
			}
		})
	}
}

func TestBeforeInitialize(t *testing.T) {
	open := `{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///early.volt"}}}`
	hover := `{"jsonrpc":"2.0","id":"h","method":"textDocument/hover","params":{}}`
	shut := `{"jsonrpc":"2.0","id":3,"method":"shutdown"}`
	s := serveString(t, frames(open, hover, shut, exitNote))
	if s.code != 1 || s.err != nil {
		t.Fatalf("exit before initialize: code %d err %v", s.code, s.err)
	}
	if len(s.calls) != 0 {
		t.Fatalf("handlers ran before initialize: %v", s.calls)
	}
	if len(s.msgs) != 2 {
		t.Fatalf("want two errors, got %v", s.raw)
	}
	expectError(t, s.msgs[0], "h", codeServerNotInitialized)
	expectError(t, s.msgs[1], 3.0, codeServerNotInitialized)
}

func TestInitializeOnce(t *testing.T) {
	s := serveString(t, frames(initReq, strings.Replace(initReq, `"id":1`, `"id":2`, 1), exitNote))
	if len(s.msgs) != 2 || s.msgs[0]["result"] == nil {
		t.Fatalf("got %v", s.raw)
	}
	expectError(t, s.msgs[1], 2.0, codeInvalidRequest)
}

func TestUnknownMethods(t *testing.T) {
	s := serveString(t, session(
		`{"jsonrpc":"2.0","id":2,"method":"textDocument/formatting","params":{}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didSave","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"$/volt","params":{}}`,
		`{"jsonrpc":"2.0","method":"$/setTrace","params":{"value":"verbose"}}`,
		`{"jsonrpc":"2.0","method":"$/cancelRequest","params":{"id":2}}`,
	))
	if len(s.msgs) != 5 {
		t.Fatalf("want initialize, register, two errors and shutdown, got %v", s.raw)
	}
	expectError(t, s.msgs[2], 2.0, codeMethodNotFound)
	expectError(t, s.msgs[3], 3.0, codeMethodNotFound)
}

func TestParamsThatDoNotDecode(t *testing.T) {
	s := serveString(t, session(
		`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":"file:///a.volt"},"position":{"line":"seven"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"textDocument/hover","params":[1,2]}`,
		`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":5}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"textDocument/hover"}`,
	))
	expectError(t, s.msgs[2], 2.0, codeInvalidParams)
	expectError(t, s.msgs[3], 3.0, codeInvalidParams)
	if s.raw[4] != `{"jsonrpc":"2.0","id":4,"result":null}` {
		t.Fatalf("absent params give the zero value and a null result: %s", s.raw[4])
	}
	for _, c := range s.calls {
		if strings.HasPrefix(c, "didOpen") {
			t.Fatalf("a notification whose params do not decode ran: %v", s.calls)
		}
	}
}

func TestUnknownFieldsIgnored(t *testing.T) {
	open := `{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///a.volt","languageId":"volt","version":1,"text":"","future":{"x":1}},"extra":true}}`
	s := serveString(t, session(open))
	if s.err != nil || s.code != 0 || len(s.calls) != 3 || s.calls[2] != "didOpen file:///a.volt" {
		t.Fatalf("calls %v, code %d, err %v", s.calls, s.code, s.err)
	}
}

func TestHandlerErrorAndIDs(t *testing.T) {
	s := serveString(t, session(
		`{"jsonrpc":"2.0","id":"r-1","method":"textDocument/rename","params":{"newName":"9x"}}`,
		`{"jsonrpc":"2.0","id":-12,"method":"textDocument/hover","params":{}}`,
		`{"jsonrpc":"2.0","id":null,"method":"textDocument/hover","params":{}}`,
		`{"jsonrpc":"2.0","id":{"n":1},"method":"textDocument/hover","params":{}}`,
		`{"jsonrpc":"1.0","id":5,"method":"textDocument/hover","params":{}}`,
		`{"jsonrpc":"2.0","id":6}`,
		`{"jsonrpc":"2.0","id":7,"method":"textDocument/hover"`,
		`[{"jsonrpc":"2.0","id":8,"method":"textDocument/hover"}]`,
	))
	expectError(t, s.msgs[2], "r-1", codeInternalError)
	if msg := s.msgs[2]["error"].(map[string]any)["message"]; msg != "rename refused: 9x" {
		t.Fatalf("the handler's error text is the message: %v", msg)
	}
	if !strings.Contains(s.raw[2], `"id":"r-1"`) || s.raw[3] != `{"jsonrpc":"2.0","id":-12,"result":null}` {
		t.Fatalf("ids are echoed as received: %s %s", s.raw[2], s.raw[3])
	}
	expectError(t, s.msgs[4], nil, codeInvalidRequest)
	expectError(t, s.msgs[5], nil, codeInvalidRequest)
	expectError(t, s.msgs[6], 5.0, codeInvalidRequest)
	expectError(t, s.msgs[7], 6.0, codeInvalidRequest)
	expectError(t, s.msgs[8], nil, codeParseError)
	expectError(t, s.msgs[9], nil, codeInvalidRequest)
	if len(s.msgs) != 11 || s.msgs[10]["id"] != 99.0 {
		t.Fatalf("every bad message gets one answer and the session goes on: %v", s.raw)
	}
}

func TestAfterShutdown(t *testing.T) {
	s := serveString(t, frames(initReq, shutdownReq,
		`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///late.volt"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"shutdown"}`,
		exitNote))
	if s.code != 0 || s.err != nil {
		t.Fatalf("code %d err %v", s.code, s.err)
	}
	if len(s.msgs) != 4 || len(s.calls) != 1 {
		t.Fatalf("messages %v, calls %v", s.raw, s.calls)
	}
	expectError(t, s.msgs[2], 2.0, codeInvalidRequest)
	expectError(t, s.msgs[3], 3.0, codeInvalidRequest)
}

func TestExitCodes(t *testing.T) {
	for name, c := range map[string]struct {
		input string
		code  int
	}{
		"exit after shutdown":              {frames(initReq, shutdownReq, exitNote), 0},
		"exit without shutdown":            {frames(initReq, exitNote), 1},
		"exit before initialize":           {frames(exitNote), 1},
		"end of input after shutdown":      {frames(initReq, shutdownReq), 0},
		"end of input without shutdown":    {frames(initReq), 1},
		"messages after exit are not read": {frames(initReq, shutdownReq, exitNote) + "garbage", 0},
	} {
		t.Run(name, func(t *testing.T) {
			if s := serveString(t, c.input); s.code != c.code || s.err != nil {
				t.Fatalf("code %d err %v, want code %d", s.code, s.err, c.code)
			}
		})
	}
}

func TestRegisterCapabilityReplyConsumed(t *testing.T) {
	hover := `{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"position":{"line":7}}}`
	for name, reply := range map[string]string{
		"result": `{"jsonrpc":"2.0","id":1,"result":null}`,
		"error":  `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"no dynamic registration"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			s := serveString(t, session(reply, hover))
			if len(s.msgs) != 4 {
				t.Fatalf("the reply is answered or lost the next request: %v", s.raw)
			}
			req := s.msgs[1]
			if req["method"] != "client/registerCapability" || req["id"] != 1.0 {
				t.Fatalf("register request: %s", s.raw[1])
			}
			regs := req["params"].(map[string]any)["registrations"].([]any)
			if len(regs) != 1 || regs[0].(map[string]any)["method"] != "workspace/didChangeWatchedFiles" {
				t.Fatalf("register params: %s", s.raw[1])
			}
			if s.msgs[2]["id"] != 2.0 || s.msgs[2]["result"] == nil {
				t.Fatalf("the request after the reply: %s", s.raw[2])
			}
		})
	}
}

func TestConcurrentNotifications(t *testing.T) {
	s := serveString(t, session(`{"jsonrpc":"2.0","id":2,"method":"burst"}`))
	seen := map[string]bool{}
	for _, m := range s.msgs {
		if m["method"] != TextDocumentPublishDiagnostics {
			continue
		}
		p := m["params"].(map[string]any)
		if d, ok := p["diagnostics"].([]any); !ok || len(d) != 0 {
			t.Fatalf("a nil diagnostics list is written as []: %v", p)
		}
		seen[p["uri"].(string)] = true
	}
	if len(seen) != 400 {
		t.Fatalf("want 400 whole notifications, got %d", len(seen))
	}
	if last := s.msgs[len(s.msgs)-2]; last["result"] != "done" {
		t.Fatalf("the burst's reply comes after its notifications: %v", last)
	}
}

func TestRequiredFieldsWritten(t *testing.T) {
	b, err := json.Marshal(PublishDiagnosticsParams{URI: "file:///a.volt"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"uri":"file:///a.volt","diagnostics":[]}` {
		t.Fatalf("clearing diagnostics: %s", b)
	}
	b, err = json.Marshal(DocumentSymbol{Name: "posts"})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"name":"posts"`, `"kind":0`, `"range":`, `"selectionRange":`} {
		if !strings.Contains(string(b), field) {
			t.Fatalf("a required field is missing from %s: %s", b, field)
		}
	}
}
