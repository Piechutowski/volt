package lsp

// Background analysis (D79). An edit is cheap to record and expensive
// to understand: the server updates the document's own front end at
// once, then analyzes the whole project once per quiet moment, on a
// goroutine of its own, through the project's lang.Session — so one
// edit costs one parse and the checks of the packages that could see
// it, however many documents are open. The run touches no Document:
// it publishes diagnostics from its own snapshot of the open texts,
// and every request adopts the newest result at its start, on the
// handler goroutine that owns the documents.

import (
	"path/filepath"
	"time"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
)

// analysisDebounce is how long the last edit must be old before the
// project is analyzed: typing stays smooth, and the result is never
// far behind the fingers.
const analysisDebounce = 75 * time.Millisecond

// analysis is one project root's background analysis: the edit
// generation it must catch up with, whether a run is under way, the
// session it loads and checks through, and the last result.
type analysis struct {
	gen     int
	running bool
	session *lang.Session
	result  *projectResult
}

// projectResult is one analysis of one project: what every open
// document of that root adopts.
type projectResult struct {
	gen     int
	root    string
	pr      *lang.Project
	diags   []diag.Diagnostic
	vindex  *voltIndex
	overlay map[string]string // the open texts the run saw
}

// projectRootOf finds the project a file belongs to (§V1.1); false for
// a relative path or a file with no go.mod above it.
func projectRootOf(path string) (string, bool) {
	if !filepath.IsAbs(path) {
		return "", false
	}
	root, err := lang.FindRoot(filepath.Dir(path))
	return root, err == nil
}

// projectAnalyze runs the project pipeline once: load through the
// session (a fresh load without one), check, index, vet on a clean
// check. Nil when the root does not load.
func projectAnalyze(root string, overlay map[string]string, session *lang.Session) *projectResult {
	var pr *lang.Project
	var err error
	if session != nil {
		pr, err = session.Load(root, overlay)
	} else {
		pr, err = lang.LoadOverlay(root, overlay)
	}
	if err != nil {
		return nil
	}
	var diags []diag.Diagnostic
	if session != nil {
		diags = session.Check(pr)
	} else {
		diags = lang.Check(pr)
	}
	res := &projectResult{root: root, pr: pr, diags: diags, overlay: overlay}
	// After Check: it is what resolves each package's imports, which
	// the index needs to follow a `db.Post` qualifier to its package.
	res.vindex = buildVoltIndex(pr, overlay)
	// Vet advice only on top of a clean check, matching the single-file
	// policy: style notes stacked on hard errors are noise while typing.
	if !diag.HasErrors(diags) {
		res.diags = append(res.diags, lang.Vet(pr)...)
	}
	return res
}

// packageOf returns the loaded package holding path; nil when the
// loader never read it (it reads only .volt files and skips dot,
// underscore and nested-module directories, §V1.6).
func (res *projectResult) packageOf(path string) *lang.Package {
	for _, pkg := range res.pr.Packages {
		for _, f := range pkg.Files {
			if f.Name == path {
				return pkg
			}
		}
	}
	return nil
}

// docDiags returns the diagnostics positioned in path, sorted. A
// conflict with another file still shows here whenever this file holds
// one of its ends, because the checker names both positions. go.mod is
// never open in an editor, so its problems (which the messages name
// explicitly) surface at the top of every project file instead of
// nowhere.
func (res *projectResult) docDiags(path string) []diag.Diagnostic {
	modPath := filepath.Join(res.root, lang.ModFile)
	var mine []diag.Diagnostic
	for _, dg := range res.diags {
		switch dg.Pos.Filename {
		case path:
			mine = append(mine, dg)
		case modPath:
			dg.Pos = token.Position{Filename: path, Line: 1, Column: 1}
			mine = append(mine, dg)
		}
	}
	diag.Sort(mine)
	return mine
}

// analysisFor returns the root's analysis state, made on first use.
// s.mu must be held.
func (s *Server) analysisFor(root string) *analysis {
	a := s.analyses[root]
	if a == nil {
		a = &analysis{session: &lang.Session{}}
		s.analyses[root] = a
	}
	return a
}

// sessionFor returns the caches of a project root, for the documents'
// own project pass.
func (s *Server) sessionFor(root string) *lang.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.analysisFor(root).session
}

// analysisKick records an edit under root and starts a run when none
// is under way; a running one picks the edit up before it finishes.
func (s *Server) analysisKick(ctx *glsp.Context, root string) {
	s.mu.Lock()
	a := s.analysisFor(root)
	a.gen++
	start := !a.running
	a.running = true
	s.mu.Unlock()
	if start {
		go s.analysisRun(ctx, root, a)
	}
}

// analysisRun is the root's background loop: wait for the edits to
// settle, analyze the open texts as they stood, publish, and go again
// if edits arrived meanwhile.
func (s *Server) analysisRun(ctx *glsp.Context, root string, a *analysis) {
	for {
		var gen int
		for {
			s.mu.Lock()
			gen = a.gen
			s.mu.Unlock()
			time.Sleep(s.debounce)
			s.mu.Lock()
			quiet := a.gen == gen
			s.mu.Unlock()
			if quiet {
				break
			}
		}
		res := projectAnalyze(root, s.openTexts(), a.session)
		s.mu.Lock()
		if res != nil {
			res.gen = gen
			a.result = res
		}
		done := a.gen == gen
		if done {
			a.running = false
		}
		s.mu.Unlock()
		if res != nil {
			s.publishResult(ctx, res)
		}
		if done {
			return
		}
	}
}

// publishResult sends every open document of the result's project its
// diagnostics, positioned in the text the run saw. A document the
// loader never read gets its own single-file verdict.
func (s *Server) publishResult(ctx *glsp.Context, res *projectResult) {
	s.mu.Lock()
	uris := make([]string, 0, len(s.docs))
	for uri := range s.docs {
		uris = append(uris, uri)
	}
	s.mu.Unlock()
	for _, uri := range uris {
		path := pathFromURI(uri)
		text, open := res.overlay[path]
		if !open {
			continue // opened after the snapshot: the next run covers it
		}
		var ds []diag.Diagnostic
		if res.packageOf(path) != nil {
			ds = res.docDiags(path)
		} else if root, ok := projectRootOf(path); ok && root == res.root {
			_, _, ds = localAnalyze(path, text)
		} else {
			continue // another project's document
		}
		ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: diagnosticsLSP(text, lineStarts(text), ds),
		})
	}
}

// adopt gives a document the newest result of its project — package,
// index and diagnostics — on the handler goroutine, before a request
// reads them.
func (s *Server) adopt(doc *Document) {
	if doc == nil {
		return
	}
	path := pathFromURI(doc.URI)
	root, ok := projectRootOf(path)
	if !ok {
		return
	}
	s.mu.Lock()
	var res *projectResult
	if a := s.analyses[root]; a != nil {
		res = a.result
	}
	s.mu.Unlock()
	if res == nil || res.gen == doc.resGen {
		return
	}
	doc.resGen = res.gen
	if pkg := res.packageOf(path); pkg != nil {
		doc.vpkg, doc.vindex, doc.Diags = pkg, res.vindex, res.docDiags(path)
	} else {
		doc.vpkg, doc.vindex, doc.Diags = nil, nil, doc.local
	}
}
