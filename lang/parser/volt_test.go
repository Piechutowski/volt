package parser

import (
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/ast"
)

const voltSample = `package app

import (
	db
	d2 shared/dicts
)

Pipeline api {
	use volt.RequestID
	use BearerAuth
}

Scope / [pipe: api, error_handler: Errors] {
	get  /            Home.Index [name: root]
	get  /about       Home.About

	Scope /admin [name: admin] {
		get /stats  Admin.Stats
	}

	resources users [api, model: db.User, only: (index, show)]

	get /files/:path...        Files.Serve
	get /users/:id(int64)/edit  Users.Edit
}
`

func TestVoltSampleParses(t *testing.T) {
	f, diags := ParseFile("app.volt", voltSample)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	// Declaration inventory, in order.
	kinds := []string{}
	for _, d := range f.Decls {
		switch d.(type) {
		case *ast.PackageClause:
			kinds = append(kinds, "package")
		case *ast.ImportDecl:
			kinds = append(kinds, "import")
		case *ast.Pipeline:
			kinds = append(kinds, "pipeline")
		case *ast.Scope:
			kinds = append(kinds, "scope")
		default:
			kinds = append(kinds, "other")
		}
	}
	if got, want := strings.Join(kinds, ","), "package,import,pipeline,scope"; got != want {
		t.Fatalf("decls = %s, want %s", got, want)
	}

	imp := f.Decls[1].(*ast.ImportDecl)
	if len(imp.Specs) != 2 {
		t.Fatalf("imports = %d, want 2", len(imp.Specs))
	}
	if q := imp.Specs[0].Qualifier(); q != "db" {
		t.Errorf("spec[0] qualifier = %q, want db", q)
	}
	if p := imp.Specs[1].PathString(); p != "shared/dicts" {
		t.Errorf("spec[1] path = %q, want shared/dicts", p)
	}
	if q := imp.Specs[1].Qualifier(); q != "d2" {
		t.Errorf("spec[1] qualifier = %q, want alias d2", q)
	}

	pl := f.Decls[2].(*ast.Pipeline)
	if len(pl.Plugs) != 2 || pl.Plugs[0].Ref.String() != "volt.RequestID" || pl.Plugs[1].Ref.String() != "BearerAuth" {
		t.Errorf("pipeline plugs wrong: %+v", pl.Plugs)
	}

	sc := f.Decls[3].(*ast.Scope)
	if sc.Path.String() != "/" {
		t.Errorf("scope path = %q, want /", sc.Path.String())
	}
	if sc.Settings.Get("pipe") == nil || sc.Settings.Get("error_handler") == nil {
		t.Errorf("scope settings missing: %v", sc.Settings)
	}
	if len(sc.Items) != 6 {
		t.Fatalf("scope items = %d, want 6", len(sc.Items))
	}
	if _, ok := sc.Items[2].(*ast.Scope); !ok {
		t.Errorf("item 2 should be the nested Scope, got %T", sc.Items[2])
	}
	res, ok := sc.Items[3].(*ast.Resources)
	if !ok {
		t.Fatalf("item 3 should be resources, got %T", sc.Items[3])
	}
	only, ok := res.Settings.Get("only").Value.(*ast.IdentList)
	if !ok || len(only.Names) != 2 {
		t.Errorf("only: identifier list wrong: %v", res.Settings.Get("only"))
	}

	wild := sc.Items[4].(*ast.Route)
	if wild.Path.String() != "/files/:path..." {
		t.Errorf("wildcard path = %q", wild.Path.String())
	}
	if wild.Path.Segments[1].Kind != ast.SegWild {
		t.Errorf("segment kind = %v, want SegWild", wild.Path.Segments[1].Kind)
	}
	typed := sc.Items[5].(*ast.Route)
	if typed.Path.String() != "/users/:id(int64)/edit" {
		t.Errorf("typed path = %q", typed.Path.String())
	}
	seg := typed.Path.Segments[1]
	if seg.Kind != ast.SegParam || seg.Name.Name() != "id" || seg.Type == nil || seg.Type.Name() != "int64" {
		t.Errorf("typed param segment wrong: %+v", seg)
	}
}

// Errors the grammar must reject, each with the spec section in the message.
func TestVoltSyntaxErrors(t *testing.T) {
	cases := []struct {
		name, src, wantMsg string
	}{
		{"space in path", "Scope / {\n\tget /users /:id Users.Show\n}\n", "expected identifier in route handler"},
		{"one-part handler", "Scope / {\n\tget /users Show\n}\n", "route handler must be Controller.Action"},
		{"dataset reserved", "Dataset da { }\n", "reserved for a future version"},
		{"import without parens", "import db\n", "expected '(' after import"},
		{"top-level route", "get /users Users.Index\n", "must appear inside a Scope"},
		{"colon without name", "Scope / {\n\tget /users/: Users.Show\n}\n", "':' must be followed by a parameter name"},
		{"short ellipsis", "Scope / {\n\tget /files/:path.. Files.Serve\n}\n", "wildcard ellipsis is spelled"},
		{"typed wildcard", "Scope / {\n\tget /files/:path(int64)... Files.Serve\n}\n", "cannot carry a type"},
		{"empty import block", "import (\n)\n", "empty import declaration"},
		{"space in import path", "import (\n\tshared / dicts\n)\n", "expected end of line after import specifier"},
		{"plug without use", "Pipeline api {\n\tRequestID\n}\n", "expected 'use' plug line"},
		{"trailing slash", "Scope / {\n\tget /users/ Users.Index\n}\n", "without a trailing slash"},
		{"space in type annotation", "Scope / {\n\tget /users/:id( int32 ) Users.Show\n}\n", "with no spaces"},
		{"space before closing paren", "Scope / {\n\tget /users/:id(int32 ) Users.Show\n}\n", "with no spaces"},
		{"quoted import path", "import (\n\t\"db\"\n)\n", "plain (unquoted) identifiers"},
		{"quoted import alias", "import (\n\t\"d\" shared/db\n)\n", "plain (unquoted) identifier"},
		{"trailing slash on scope", "Scope /admin/ {\n\tget / A.Index\n}\n", "without a trailing slash"},
		// An empty segment ("//") is lexically unreachable — '//' opens a
		// line comment (§V0.2) — so the non-segment case needs another
		// token abutting the slash. (Quoted segments parse as identifiers
		// and are rejected by the checker, §V4.1.6.)
		{"non-segment after slash", "Scope / {\n\tget /users/[name: root] Users.Index\n}\n", "'/' must be followed by a path segment"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, diags := ParseFile("t.volt", tc.src)
			if len(diags) == 0 {
				t.Fatalf("expected a diagnostic for %q", tc.src)
			}
			found := false
			for _, d := range diags {
				if strings.Contains(d.Msg, tc.wantMsg) {
					found = true
				}
			}
			if !found {
				t.Errorf("diagnostics %v do not mention %q", diags, tc.wantMsg)
			}
		})
	}
}

// One broken plug line must not abort the Pipeline: the parser recovers
// to the next line, reports each bad plug, and keeps the good ones.
func TestPipelineRecoversPerPlug(t *testing.T) {
	src := "Pipeline api {\n\tuse volt.RequestID\n\tRequestID\n\tBearerAuth extra\n\tuse Last\n}\n"
	f, diags := ParseFile("p.volt", src)
	if len(diags) != 2 {
		t.Fatalf("diagnostics = %d (%v), want 2 — one per bad plug line", len(diags), diags)
	}
	if len(f.Decls) != 1 {
		t.Fatalf("decls = %d, want 1 (the Pipeline must survive)", len(f.Decls))
	}
	pl, ok := f.Decls[0].(*ast.Pipeline)
	if !ok {
		t.Fatalf("decl is %T, want *ast.Pipeline", f.Decls[0])
	}
	if len(pl.Plugs) != 2 || pl.Plugs[0].Ref.String() != "volt.RequestID" || pl.Plugs[1].Ref.String() != "Last" {
		t.Errorf("plugs = %+v, want the two good ones", pl.Plugs)
	}
}

// A pure-DBML file must parse exactly as before: the Volt layer is a
// strict superset (§V0).
func TestVoltLayerIsSuperset(t *testing.T) {
	src := "Table users {\n\tid integer [pk]\n}\n"
	f, diags := ParseFile("s.volt", src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Decls) != 1 {
		t.Fatalf("decls = %d, want 1", len(f.Decls))
	}
	if _, ok := f.Decls[0].(*ast.Table); !ok {
		t.Fatalf("decl is %T, want *ast.Table", f.Decls[0])
	}
}
