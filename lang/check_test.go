package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/nao/edbml/diag"
)

// project materializes files into a temp dir and runs Load+Check.
func project(t *testing.T, files map[string]string) (*Project, []diag.Diagnostic) {
	t.Helper()
	root := t.TempDir()
	for name, src := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pr, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return pr, Check(pr)
}

func wantClean(t *testing.T, diags []diag.Diagnostic) {
	t.Helper()
	if diag.HasErrors(diags) {
		t.Fatalf("unexpected errors: %v", diags)
	}
}

func wantError(t *testing.T, diags []diag.Diagnostic, substr string) {
	t.Helper()
	for _, d := range diags {
		if strings.Contains(d.Msg, substr) {
			return
		}
	}
	t.Fatalf("no diagnostic mentions %q; got %v", substr, diags)
}

const modFile = "module blog\n"

// The reference project: a small blog — a schema package and a routes
// package exercising imports, model inference, resources, nesting and
// helpers.
var blog = map[string]string{
	"volt.mod": modFile,
	"db/schema.volt": `package db

Table users {
	id    integer [pk, increment]
	email text    [not null, unique]
}

Table posts {
	id    integer [pk, increment]
	title text    [not null]
}
`,
	"app/routes.volt": `package app

import (
	db
)

Pipeline api {
	use volt.RequestID
	use BearerAuth
}

Scope / [pipe: api, error_handler: Errors] {
	get  /        Home.Index [name: root]
	get  /about   Home.About

	Scope /admin [name: admin] {
		get /stats Admin.Stats
	}

	resources users [model: db.User, only: (index, show, create)]

	get /files/:path...       Files.Serve
	get /users/:id(int32)/avatar  Users.Avatar
}
`,
}

func TestReferenceProject(t *testing.T) {
	pr, diags := project(t, blog)
	wantClean(t, diags)

	app := pr.Packages["app"]
	if app == nil {
		t.Fatal("package app not loaded")
	}
	if !app.HasRouting() {
		t.Error("app should have routing")
	}
	if pr.Packages["db"].HasRouting() {
		t.Error("db should not have routing")
	}

	// Route inventory. resources users with only:(index,show,create)
	// yields 3 routes; 5 plain routes total.
	if len(app.Routes) != 8 {
		for _, r := range app.Routes {
			t.Logf("route: %s", r)
		}
		t.Fatalf("routes = %d, want 8", len(app.Routes))
	}

	byPattern := map[string]*RouteInfo{}
	for _, r := range app.Routes {
		byPattern[r.Method+" "+r.Pattern] = r
	}

	root := byPattern["GET /{$}"]
	if root == nil || root.HelperName != "Root" {
		t.Errorf("root route wrong: %+v", root)
	}
	stats := byPattern["GET /admin/stats"]
	if stats == nil || stats.HelperName != "AdminStats" {
		t.Errorf("scope-name prefixing wrong: %+v", stats)
	}
	show := byPattern["GET /users/{id}"]
	if show == nil {
		t.Fatal("resources show route missing")
	}
	if show.Params[0].Type != TInt32 {
		t.Errorf("model-inferred key type = %v, want int32 (integer pk)", show.Params[0].Type)
	}
	if show.Controller != "Users" || show.Action != "Show" || show.HelperName != "User" {
		t.Errorf("show route wrong: %+v", show)
	}
	wild := byPattern["GET /files/{path...}"]
	if wild == nil || !wild.Params[0].Wild || wild.Params[0].Type != TString {
		t.Errorf("wildcard route wrong: %+v", wild)
	}
	avatar := byPattern["GET /users/{id}/avatar"]
	if avatar == nil || avatar.Params[0].Type != TInt32 {
		t.Errorf("typed param route wrong: %+v", avatar)
	}

	// Pipelines and error handler flow down.
	for _, r := range app.Routes {
		if len(r.Pipes) != 1 || r.Pipes[0] != "api" {
			t.Errorf("route %s pipes = %v, want [api]", r.Pattern, r.Pipes)
		}
		if r.ErrorHandler != "Errors" {
			t.Errorf("route %s error handler = %q, want Errors", r.Pattern, r.ErrorHandler)
		}
	}

	// Controller signatures.
	users := app.Controllers["Users"]
	if users == nil {
		t.Fatal("Users controller missing")
	}
	if a := users.Action("Show"); a == nil || len(a.Params) != 1 || a.Params[0].Type != TInt32 {
		t.Errorf("Users.Show signature wrong: %+v", users)
	}
	if a := users.Action("Index"); a == nil || len(a.Params) != 0 {
		t.Errorf("Users.Index signature wrong")
	}
}

func TestUpdateSpansPatchAndPut(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"volt.mod": modFile,
		"app/r.volt": `package app

Scope / {
	resources users [api]
}
`,
	})
	wantClean(t, diags)
	app := pr.Packages["app"]
	// api: index, create, show, update(PATCH+PUT), delete = 6 routes
	if len(app.Routes) != 6 {
		t.Fatalf("routes = %d, want 6", len(app.Routes))
	}
	methods := map[string]bool{}
	for _, r := range app.Routes {
		if r.Action == "Update" {
			methods[r.Method] = true
		}
	}
	if !methods["PATCH"] || !methods["PUT"] {
		t.Errorf("update methods = %v, want PATCH and PUT", methods)
	}
	// One Update action despite two routes.
	if a := app.Controllers["Users"].Action("Update"); a == nil || len(a.Routes) != 2 {
		t.Errorf("Update action should carry both routes")
	}
}

func TestResourcesParamExceptNoModel(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"volt.mod": modFile,
		"app/r.volt": `package app

Scope / {
	resources posts [param: slug, except: (delete)]
}
`,
	})
	wantClean(t, diags)
	app := pr.Packages["app"]
	// Full set is 8 routes (7 actions, Update spanning PATCH+PUT);
	// except:(delete) removes one.
	if len(app.Routes) != 7 {
		for _, r := range app.Routes {
			t.Logf("route: %s", r)
		}
		t.Fatalf("routes = %d, want 7", len(app.Routes))
	}
	for _, r := range app.Routes {
		if r.Action == "Delete" {
			t.Errorf("excepted action Delete still expanded: %s", r)
		}
		for _, p := range r.Params {
			if p.Name != "slug" {
				t.Errorf("param: rename not applied on %s: %q", r.Pattern, p.Name)
			}
			if p.Type != TInt64 {
				t.Errorf("no-model key type = %v on %s, want int64 (§V5.2)", p.Type, r.Pattern)
			}
		}
		if strings.Contains(r.Pattern, "{") && !strings.Contains(r.Pattern, "{slug}") {
			t.Errorf("keyed pattern wrong: %s", r.Pattern)
		}
	}
}

func TestResourcesSingularOverride(t *testing.T) {
	// A name English singularization leaves alone collides with itself…
	_, diags := project(t, map[string]string{
		"volt.mod":   modFile,
		"app/r.volt": "package app\n\nScope / {\n\tresources posty\n}\n",
	})
	wantError(t, diags, "set the singular explicitly")

	// …and singular: is the fix, giving distinct collection/member helpers.
	pr, diags := project(t, map[string]string{
		"volt.mod":   modFile,
		"app/r.volt": "package app\n\nScope / {\n\tresources posty [singular: post]\n}\n",
	})
	wantClean(t, diags)
	helpers := map[string]bool{}
	for _, r := range pr.Packages["app"].Routes {
		if r.HelperName != "" {
			helpers[r.HelperName] = true
		}
	}
	for _, want := range []string{"Posty", "Post", "NewPost", "EditPost"} {
		if !helpers[want] {
			t.Errorf("helper %q missing; got %v", want, helpers)
		}
	}
}

func TestVerbMethodMapping(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"volt.mod": modFile,
		"app/r.volt": `package app

Scope / {
	get     /a  C.A
	post    /b  C.B
	put     /c  C.C
	patch   /d  C.D
	delete  /e  C.E
	options /f  C.F
	head    /g  C.G
	any     /h  C.H
}
`,
	})
	wantClean(t, diags)
	want := map[string]string{
		"/a": "GET", "/b": "POST", "/c": "PUT", "/d": "PATCH",
		"/e": "DELETE", "/f": "OPTIONS", "/g": "HEAD", "/h": "",
	}
	app := pr.Packages["app"]
	if len(app.Routes) != len(want) {
		t.Fatalf("routes = %d, want %d", len(app.Routes), len(want))
	}
	for _, r := range app.Routes {
		if m, ok := want[r.Pattern]; !ok || r.Method != m {
			t.Errorf("%s: Method = %q, want %q (any maps to the empty method)", r.Pattern, r.Method, want[r.Pattern])
		}
	}
}

func TestVetUnusedPipeline(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"volt.mod": modFile,
		"app/r.volt": `package app

Pipeline api {
	use volt.RequestID
}

Pipeline dead {
	use volt.Logger
}

Scope / [pipe: api] {
	get / Home.Index
}
`,
	})
	wantClean(t, diags)
	warns := Vet(pr)
	if len(warns) != 1 || !strings.Contains(warns[0].Msg, `"dead"`) {
		t.Fatalf("Vet = %v, want exactly one unused-pipeline warning for dead", warns)
	}
}

func TestErrors(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"missing package clause", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "Scope / {\n\tget /x X.Y\n}\n",
		}, "must begin with a package clause"},
		{"package/dir mismatch", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package routes\n",
		}, "must match its directory name"},
		{"file name disagreement", map[string]string{
			"volt.mod":   modFile,
			"app/a.volt": "package app\n",
			"app/b.volt": "package app2\n",
		}, "disagrees"},
		{"use rejected", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nuse * from './x'\n",
		}, "not part of the Volt language"},
		{"unknown import", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nimport (\n\tnothere\n)\nScope / [pipe: p] { get /x X.Y }\n",
		}, "unknown package"},
		{"self import", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nimport (\n\tapp\n)\n",
		}, "cannot import itself"},
		{"unused import", map[string]string{
			"volt.mod":   modFile,
			"db/s.volt":  "package db\nTable users { id integer [pk] }\n",
			"app/r.volt": "package app\nimport (\n\tdb\n)\n",
		}, "imported and not used"},
		{"import cycle", map[string]string{
			"volt.mod":  modFile,
			"a/a.volt":  "package a\nimport (\n\tb\n)\nScope / { resources users [model: b.User] }\n",
			"b/b.volt":  "package b\nimport (\n\ta\n)\nScope /b { resources users [model: a.User] }\n",
			"a/t.volt":  "package a\nTable users { id integer [pk] }\n",
			"b/t2.volt": "package b\nTable users { id integer [pk] }\n",
		}, "import cycle"},
		{"duplicate route", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nScope / {\n\tget /users/:id Users.Show\n\tget /users/:name Users.ByName\n}\n",
		}, "identical method and path shape"},
		{"helper collision", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nScope / {\n\tget /a A.Show\n\tget /b B.Show [name: show]\n}\n",
		}, "already produced by the route"},
		{"signature conflict", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nScope / {\n\tget /a/:id(int64) X.Show\n\tget /b/:id(int32) X.Show\n}\n",
		}, "different parameter signatures"},
		{"wildcard not last", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nScope / {\n\tget /f/:p.../x F.S\n}\n",
		}, "must be the last path segment"},
		{"wildcard in scope prefix", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nScope /f/:p... {\n\tget /x F.S\n}\n",
		}, "Scope prefix cannot contain a wildcard"},
		{"unknown pipeline", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nScope / [pipe: nope] {\n\tget /x X.Y\n}\n",
		}, "unknown Pipeline"},
		{"unknown param type", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nScope / {\n\tget /x/:id(uuid) X.Y\n}\n",
		}, "unknown parameter type"},
		{"duplicate param", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nScope /u/:id {\n\tget /p/:id X.Y\n}\n",
		}, "duplicate path parameter"},
		{"unknown model", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nScope / {\n\tresources users [model: Nope]\n}\n",
		}, "no table in package"},
		{"only and except", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nScope / {\n\tresources users [only: (index), except: (show)]\n}\n",
		}, "cannot be combined"},
		{"unknown action", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nScope / {\n\tresources users [only: (browse)]\n}\n",
		}, "unknown action"},
		{"lowercase controller", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nScope / {\n\tget /x home.Index\n}\n",
		}, "exported Go identifiers"},
		{"scope setting typo", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nScope / [pipes: api] {\n\tget /x X.Y\n}\n",
		}, "not valid on a Scope"},
		{"keyword param", map[string]string{
			"volt.mod":   modFile,
			"app/r.volt": "package app\nScope / {\n\tget /x/:type X.Y\n}\n",
		}, "non-keyword"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, diags := project(t, tc.files)
			wantError(t, diags, tc.want)
		})
	}
}

func TestFileLayoutIsInvisible(t *testing.T) {
	// L1: the same declarations in one file or three must yield the same
	// routes in the same order.
	one, d1 := project(t, map[string]string{
		"volt.mod": modFile,
		"app/all.volt": `package app
Table users { id integer [pk] }
Pipeline api { use volt.RequestID }
Scope / [pipe: api] {
	resources users [model: User]
	get /about Home.About
}
`,
	})
	wantClean(t, d1)
	three, d3 := project(t, map[string]string{
		"volt.mod":     modFile,
		"app/a_s.volt": "package app\nTable users { id integer [pk] }\n",
		"app/b_p.volt": "package app\nPipeline api { use volt.RequestID }\n",
		"app/c_r.volt": "package app\nScope / [pipe: api] {\n\tresources users [model: User]\n\tget /about Home.About\n}\n",
	})
	wantClean(t, d3)

	r1, r3 := one.Packages["app"].Routes, three.Packages["app"].Routes
	if len(r1) != len(r3) {
		t.Fatalf("route counts differ: %d vs %d", len(r1), len(r3))
	}
	for i := range r1 {
		if r1[i].Method != r3[i].Method || r1[i].Pattern != r3[i].Pattern || r1[i].HelperName != r3[i].HelperName {
			t.Errorf("route %d differs: %s vs %s", i, r1[i], r3[i])
		}
	}
}
