package lsp

import (
	"path/filepath"
	"time"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/token"
)

const analysisDebounce = 75 * time.Millisecond

type analysis struct {
	gen     int
	running bool
	session *lang.Session
	index   voltIndexMemo
	result  *projectResult
}

type projectResult struct {
	gen      int
	root     string
	pr       *lang.Project
	diags    []diag.Diagnostic
	vindex   *voltIndex
	overlay  map[string]string
	versions map[string]int32
}

func projectRootOf(path string) (string, bool) {
	if !filepath.IsAbs(path) {
		return "", false
	}
	root, err := lang.FindRoot(filepath.Dir(path))
	return root, err == nil
}

func projectAnalyze(root string, overlay map[string]string, session *lang.Session, memo *voltIndexMemo) *projectResult {
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
	res.vindex = buildVoltIndex(pr, overlay, memo)
	if !diag.HasErrors(diags) {
		if session != nil {
			res.diags = append(res.diags, session.Vet(pr)...)
		} else {
			res.diags = append(res.diags, lang.Vet(pr)...)
		}
	}
	return res
}

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

func (res *projectResult) docDiags(path string) []diag.Diagnostic {
	modPath := filepath.Join(res.root, lang.ModFile)
	var mine []diag.Diagnostic
	for _, dg := range res.diags {
		switch dg.Pos.Filename() {
		case path:
			mine = append(mine, dg)
		case modPath:
			dg.Pos = token.At(path, 0, 1, 1)
			mine = append(mine, dg)
		}
	}
	diag.Sort(mine)
	return mine
}

func (s *Server) analysisFor(root string) *analysis {
	a := s.analyses[root]
	if a == nil {
		a = &analysis{session: &lang.Session{}}
		s.analyses[root] = a
	}
	return a
}

func (s *Server) sessionFor(root string) *lang.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.analysisFor(root).session
}

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
		texts, versions := s.openSnapshot()
		res := projectAnalyze(root, texts, a.session, &a.index)
		s.mu.Lock()
		if res != nil {
			res.gen, res.versions = gen, versions
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
			continue
		}
		var ds []diag.Diagnostic
		if res.packageOf(path) != nil {
			ds = res.docDiags(path)
		} else if root, ok := projectRootOf(path); ok && root == res.root {
			ds = localVerdict(path, text)
		} else {
			continue
		}
		ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
			URI:         uri,
			Version:     versionStamp(res.versions[path]),
			Diagnostics: diagnosticsLSP(text, lineStarts(text), ds),
		})
	}
}

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
		doc.vpkg, doc.vindex = nil, nil
		doc.vetLocal()
		doc.Diags = doc.local
	}
}
