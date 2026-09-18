package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Piechutowski/volt/lsp"
)

// The language server, driven the way an editor drives it: a scripted
// stdio JSON-RPC session against the real server process, replaying
// keystrokes, with every response and notification pinned (D101).
// What unit tests of the handlers miss, this catches: the framing,
// the background analysis's publish after an edit, what an edit that
// breaks a file reports and what fixing it reports.

var updateLSP = flag.Bool("update", false, "rewrite the language server session golden")

// TestLSPServe is the server process: the session test runs this test
// binary again with the marker set and speaks to it over its pipes.
func TestLSPServe(t *testing.T) {
	if os.Getenv("VOLT_LSP_SERVE") != "1" {
		t.Skip("the server side of TestLSPSession")
	}
	if err := lsp.NewServer().RunStdio(); err != nil {
		t.Fatal(err)
	}
}

type lspSession struct {
	t      *testing.T
	stdin  io.WriteCloser
	msgs   chan map[string]any
	id     int
	root   string
	script strings.Builder
}

func (s *lspSession) send(method string, params any, request bool) int {
	m := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	id := 0
	if request {
		s.id++
		id = s.id
		m["id"] = id
	}
	b, err := json.Marshal(m)
	if err != nil {
		s.t.Fatal(err)
	}
	fmt.Fprintf(s.stdin, "Content-Length: %d\r\n\r\n%s", len(b), b)
	return id
}

// record writes a message to the script, keys sorted by the JSON
// encoder, the project root spelled ROOT.
func (s *lspSession) record(label string, m map[string]any) {
	b, err := json.Marshal(m)
	if err != nil {
		s.t.Fatal(err)
	}
	text := strings.ReplaceAll(string(b), s.root, "ROOT")
	fmt.Fprintf(&s.script, "%s\n%s\n\n", label, text)
}

// next waits for the next message from the server.
func (s *lspSession) next() map[string]any {
	select {
	case m, ok := <-s.msgs:
		if !ok {
			s.t.Fatal("the server closed its output")
		}
		return m
	case <-time.After(30 * time.Second):
		s.t.Fatal("no message from the server in 30s")
	}
	return nil
}

// response waits for the response to a request, recording every
// notification that arrives before it.
func (s *lspSession) response(label string, id int) {
	for {
		m := s.next()
		if got, ok := m["id"]; ok && got == float64(id) {
			s.record(label, m)
			return
		}
		s.record(label+" (before the response)", m)
	}
}

// notification waits for a notification of the method.
func (s *lspSession) notification(label, method string) {
	for {
		m := s.next()
		s.record(label, m)
		if m["method"] == method {
			return
		}
	}
}

func TestLSPSession(t *testing.T) {
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module session\n\ngo 1.27\n",
		"app/schema.volt": `package app

Table posts {
	id integer [pk, increment]
	title text [not null]
}

Scope / {
	get / Home.Index
	resources posts
}
`,
	}
	for name, text := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestLSPServe$")
	cmd.Env = append(os.Environ(), "VOLT_LSP_SERVE=1")
	cmd.Dir = root
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Wait()
	s := &lspSession{t: t, stdin: stdin, msgs: make(chan map[string]any, 100), root: root}
	go func() {
		rd := bufio.NewReader(stdout)
		defer close(s.msgs)
		for {
			length := 0
			for {
				line, err := rd.ReadString('\n')
				if err != nil {
					return
				}
				line = strings.TrimSpace(line)
				if line == "" {
					break
				}
				if strings.HasPrefix(line, "Content-Length: ") {
					length, _ = strconv.Atoi(strings.TrimPrefix(line, "Content-Length: "))
				}
			}
			body := make([]byte, length)
			if _, err := io.ReadFull(rd, body); err != nil {
				return
			}
			var m map[string]any
			if err := json.Unmarshal(body, &m); err != nil {
				return
			}
			s.msgs <- m
		}
	}()

	path := filepath.Join(root, "app", "schema.volt")
	uri := "file://" + path
	text := files["app/schema.volt"]
	doc := func(version int) map[string]any {
		return map[string]any{"uri": uri, "version": version}
	}
	change := func(label string, version int, edited string) {
		s.send("textDocument/didChange", map[string]any{"textDocument": doc(version),
			"contentChanges": []map[string]any{{"text": edited}}}, false)
		s.notification(label, "textDocument/publishDiagnostics")
	}
	// typed sends one keystroke the way an incremental-sync client
	// does (D105): the text inserted at a position, not the file.
	typed := func(label string, version, line, character int, inserted string) {
		at := map[string]any{"line": line, "character": character}
		s.send("textDocument/didChange", map[string]any{"textDocument": doc(version),
			"contentChanges": []map[string]any{{"range": map[string]any{"start": at, "end": at}, "text": inserted}}}, false)
		s.notification(label, "textDocument/publishDiagnostics")
	}

	id := s.send("initialize", map[string]any{"processId": os.Getpid(), "rootUri": "file://" + root,
		"capabilities": map[string]any{}}, true)
	s.response("initialize", id)
	s.send("initialized", map[string]any{}, false)
	s.send("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "volt", "version": 1, "text": text}}, false)
	s.notification("open: a clean file", "textDocument/publishDiagnostics")

	// keystrokes: the settings bracket goes missing, then comes back
	// with a second table whose route has a typo, then all is well
	broken := strings.Replace(text, "[pk, increment]", "[pk, increment", 1)
	change("edit 1: a settings list left open", 2, broken)
	typo := strings.Replace(text, "get / Home.Index", "gett / Home.Index", 1) + "\nTable tags {\n\tid integer [pk]\n}\n"
	change("edit 2: a verb misspelled, a table added", 3, typo)
	change("edit 3: the verb fixed", 4, strings.Replace(typo, "gett /", "get /", 1))
	typed("edit 4: a note typed into the new table's header, as a ranged change", 5, 12, 10, " [note: 'tagging']")

	// requests on the fixed file
	id = s.send("textDocument/hover", map[string]any{"textDocument": map[string]any{"uri": uri}, "position": map[string]any{"line": 2, "character": 7}}, true)
	s.response("hover on the table name", id)
	id = s.send("textDocument/hover", map[string]any{"textDocument": map[string]any{"uri": uri}, "position": map[string]any{"line": 9, "character": 12}}, true)
	s.response("hover on the resources table", id)
	id = s.send("textDocument/definition", map[string]any{"textDocument": map[string]any{"uri": uri}, "position": map[string]any{"line": 9, "character": 12}}, true)
	s.response("definition from the resources table", id)
	id = s.send("textDocument/completion", map[string]any{"textDocument": map[string]any{"uri": uri}, "position": map[string]any{"line": 3, "character": 25}}, true)
	s.response("completion inside the settings list", id)
	id = s.send("textDocument/documentSymbol", map[string]any{"textDocument": map[string]any{"uri": uri}}, true)
	s.response("document symbols", id)
	id = s.send("shutdown", nil, true)
	s.response("shutdown", id)
	s.send("exit", nil, false)
	stdin.Close()

	golden := filepath.Join("testdata", "lsp_session.golden")
	got := s.script.String()
	if *updateLSP {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run with -update to write it)", err)
	}
	if string(want) != got {
		wl, gl := strings.Split(string(want), "\n"), strings.Split(got, "\n")
		n := len(gl)
		if len(wl) < n {
			n = len(wl)
		}
		for i := 0; i < n; i++ {
			if wl[i] != gl[i] {
				t.Fatalf("session differs at line %d:\n--- golden\n%s\n--- got\n%s\n(run with -update after reading the diff)", i+1, wl[i], gl[i])
			}
		}
		t.Fatalf("session differs in length: golden %d lines, got %d (run with -update after reading the diff)", len(wl), len(gl))
	}
}
