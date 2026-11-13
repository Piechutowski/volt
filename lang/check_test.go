package lang

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/internal/disk"
	"github.com/Piechutowski/volt/lang/diag"
)

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
	pr, err := Load(disk.Source{}, root)
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

var blog = map[string]string{
	"go.mod":    modFile,
	"app/mw.go": "package app\n\nimport (\n\t\"net/http\"\n\n\tvolt \"github.com/Piechutowski/volt\"\n)\n\nfunc BearerAuth(next http.Handler) http.Handler { return next }\nfunc Errors(r volt.Request, err error) volt.Response { return volt.DefaultErrorHandler(r, err) }\n",
	"db/schema.volt": `package db

table users {
	id    int [pk]
	email string    [unique]
}

table posts {
	id    int [pk]
	title string
}
`,
	"app/routes.volt": `package app

import (
	db
)

pipeline api {
	use volt.RequestID
	use BearerAuth
}

scope / [pipe: api, error_handler: Errors] {
	get  /        Home.Index [name: root]
	get  /about   Home.About

	scope /admin [name: admin] {
		get /stats Admin.Stats
	}

	resources db.users [only: (index, show, create)]

	get /files/:path...       Files.Serve
	get /users/:id(int)/avatar  Users.Avatar
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
	if show.Params[0].Type != TInt {
		t.Errorf("model-inferred key type = %v, want int (integer pk)", show.Params[0].Type)
	}
	if show.Controller != "Users" || show.Action != "Show" || show.HelperName != "User" {
		t.Errorf("show route wrong: %+v", show)
	}
	wild := byPattern["GET /files/{path...}"]
	if wild == nil || !wild.Params[0].Wild || wild.Params[0].Type != TString {
		t.Errorf("wildcard route wrong: %+v", wild)
	}
	avatar := byPattern["GET /users/{id}/avatar"]
	if avatar == nil || avatar.Params[0].Type != TInt {
		t.Errorf("typed param route wrong: %+v", avatar)
	}

	for _, r := range app.Routes {
		if len(r.Pipes) != 1 || r.Pipes[0] != "api" {
			t.Errorf("route %s pipes = %v, want [api]", r.Pattern, r.Pipes)
		}
		if r.ErrorHandler != "Errors" {
			t.Errorf("route %s error handler = %q, want Errors", r.Pattern, r.ErrorHandler)
		}
	}

	users := app.Controllers["Users"]
	if users == nil {
		t.Fatal("Users controller missing")
	}
	if a := users.Action("Show"); a == nil || len(a.Params) != 1 || a.Params[0].Type != TInt {
		t.Errorf("Users.Show signature wrong: %+v", users)
	}
	if a := users.Action("Index"); a == nil || len(a.Params) != 0 {
		t.Errorf("Users.Index signature wrong")
	}
}

func TestUpdateIsPatch(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod": modFile,
		"app/r.volt": `package app

table users {
	id int [pk]
}

scope / {
	resources users [api]
}
`,
	})
	wantClean(t, diags)
	app := pr.Packages["app"]
	if len(app.Routes) != 5 {
		t.Fatalf("routes = %d, want 5", len(app.Routes))
	}
	methods := map[string]bool{}
	for _, r := range app.Routes {
		if r.Action == "Update" {
			methods[r.Method] = true
		}
	}
	if len(methods) != 1 || !methods["PATCH"] {
		t.Errorf("update methods = %v, want PATCH alone (§the-action-table)", methods)
	}
	if a := app.Controllers["Users"].Action("Update"); a == nil || len(a.Routes) != 1 {
		t.Errorf("Update action should carry one route")
	}
}

func TestResourcesParamExcept(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod": modFile,
		"app/r.volt": `package app

table posts {
	id int [pk]
}

scope / {
	resources posts [param: slug, except: (delete)]
}
`,
	})
	wantClean(t, diags)
	app := pr.Packages["app"]
	if len(app.Routes) != 6 {
		for _, r := range app.Routes {
			t.Logf("route: %s", r)
		}
		t.Fatalf("routes = %d, want 6", len(app.Routes))
	}
	for _, r := range app.Routes {
		if r.Action == "Delete" {
			t.Errorf("excepted action Delete still expanded: %s", r)
		}
		for _, p := range r.Params {
			if p.Name != "slug" {
				t.Errorf("param: rename not applied on %s: %q", r.Pattern, p.Name)
			}
			if p.Type != TInt {
				t.Errorf("key type = %v on %s, want int from the pk (§table-resolution)", p.Type, r.Pattern)
			}
		}
		if strings.Contains(r.Pattern, "{") && !strings.Contains(r.Pattern, "{slug}") {
			t.Errorf("keyed pattern wrong: %s", r.Pattern)
		}
	}
}

func TestResourcesSingularOverride(t *testing.T) {
	_, diags := project(t, map[string]string{
		"go.mod":     modFile,
		"app/r.volt": "package app\n\ntable posty {\n\tid int [pk]\n}\n\nscope / {\n\tresources posty\n}\n",
	})
	wantError(t, diags, "set the singular explicitly")

	pr, diags := project(t, map[string]string{
		"go.mod":     modFile,
		"app/r.volt": "package app\n\ntable posty {\n\tid int [pk]\n}\n\nscope / {\n\tresources posty [singular: post]\n}\n",
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

func TestResourcesNamesTable(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod":         modFile,
		"db/schema.volt": "package db\n\ntable posty [model: \"Post\"] {\n\tid int [pk]\n}\n",
		"app/r.volt":     "package app\nimport (\n\tdb\n)\nscope / {\n\tresources db.posty\n}\n",
	})
	wantClean(t, diags)

	app := pr.Packages["app"]
	helpers := map[string]bool{}
	for _, r := range app.Routes {
		if !strings.HasPrefix(r.Pattern, "/posty") {
			t.Errorf("pattern %q should use the table name", r.Pattern)
		}
		for _, p := range r.Params {
			if p.Type != TInt {
				t.Errorf("key type = %v on %s, want int", p.Type, r.Pattern)
			}
		}
		if r.HelperName != "" {
			helpers[r.HelperName] = true
		}
	}
	for _, want := range []string{"Posty", "Post", "NewPost", "EditPost"} {
		if !helpers[want] {
			t.Errorf("helper %q missing; got %v", want, helpers)
		}
	}
	if app.Controllers["Posty"] == nil {
		t.Errorf("controller should be Posty (from the table); got %v", app.Controllers)
	}
}

func TestResourcesRequireDeclaredTable(t *testing.T) {
	_, diags := project(t, map[string]string{
		"go.mod":     modFile,
		"app/r.volt": "package app\n\ntable posts {\n\tid int [pk]\n}\n\nscope / {\n\tresources ghosts\n}\n",
	})
	wantError(t, diags, `no table "ghosts" in package "app"`)

	_, diags = project(t, map[string]string{
		"go.mod":     modFile,
		"app/r.volt": "package app\n\ntable posts {\n\tid int [pk]\n}\n\nscope / {\n\tresources Post\n}\n",
	})
	wantError(t, diags, `is the model of table "posts"`)
}

func TestVerbMethodMapping(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod": modFile,
		"app/r.volt": `package app

scope / {
	get     /a  C.A
	post    /b  C.B
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
		"/a": "GET", "/b": "POST", "/d": "PATCH",
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

func TestPutIsNoVerb(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod": modFile,
		"app/r.volt": `package app

scope / {
	put   /c C.C
	PUT   /d C.D
	patch /c C.P
}
`,
	})
	n := 0
	for _, d := range diags {
		if d.Code != "spec/routes" || !strings.Contains(d.Msg, "put is not a route verb: the partial update is patch") {
			t.Errorf("unexpected diagnostic %s", d)
			continue
		}
		n++
	}
	if n != 2 {
		t.Errorf("put errors = %d, want one per put line: %v", n, diags)
	}
	if routes := pr.Packages["app"].Routes; len(routes) != 1 || routes[0].Action != "P" {
		t.Errorf("routes = %v, want the patch route alone", routes)
	}
}

func TestPutNamesTheVerbToWrite(t *testing.T) {
	schema := "package db\n\ntable users {\n\tid int [pk]\n\temail string\n}\n\nselect picked for users where id in :ids\n"
	cases := []struct{ route, advice string }{
		{"put /x db.UserList", "db.UserList is routed with get, so write get /x db.UserList"},
		{"put /x db.UserPicked", "db.UserPicked is routed with get, so write get /x db.UserPicked"},
		{"put /x/:id(int) db.UserGet", "db.UserGet is routed with get, so write get /x/:id(int) db.UserGet"},
		{"put /x db.UserCreate", "db.UserCreate is routed with post, so write post /x db.UserCreate"},
		{"put /x/:id(int) db.UserDelete", "db.UserDelete is routed with delete, so write delete /x/:id(int) db.UserDelete"},
		{"put /x/:id(int) db.UserUpdate", "the partial update is patch, so write patch /x/:id(int) db.UserUpdate"},
		{"PUT /events volt.Events", "an event route is read with get, so write get /events volt.Events"},
		{"Put /x/:id(int) Users.Replace", "the partial update is patch, so write patch /x/:id(int) Users.Replace"},
	}
	for _, tc := range cases {
		t.Run(tc.route, func(t *testing.T) {
			files := map[string]string{
				"go.mod":         modFile,
				"db/schema.volt": schema,
				"app/r.volt":     "package app\n\nimport (\n\tdb\n)\n\nscope / {\n\tget /keep db.UserList [name: keep]\n\t" + tc.route + "\n}\n",
			}
			_, diags := project(t, files)
			want := "put is not a route verb: " + tc.advice + " (§routes)"
			if len(diags) != 1 || diags[0].Code != "spec/routes" || diags[0].Msg != want {
				t.Fatalf("want the one put error %q; got %v", want, diags)
			}
			line := tc.advice[strings.Index(tc.advice, "so write ")+len("so write "):]
			files["app/r.volt"] = strings.Replace(files["app/r.volt"], tc.route, line, 1)
			_, diags = project(t, files)
			wantClean(t, diags)
		})
	}

	_, diags := project(t, map[string]string{
		"go.mod":         modFile,
		"db/schema.volt": schema,
		"app/r.volt":     "package app\n\nimport (\n\tdb\n)\n\nscope / {\n\tput /x db.UserFetch\n\tput /y volt.Stream\n\tput /z/:slug(string) Users.Replace\n}\n",
	})
	generic := "put is not a route verb: the partial update is patch (§routes)"
	for _, want := range []string{
		"8:2: error: spec/routes: " + generic,
		"8:12: error: spec/scopes-and-routes: no generated query db.UserFetch",
		"9:2: error: spec/routes: " + generic,
		"9:9: error: spec/scopes-and-routes: the runtime provides no handler volt.Stream",
		"10:2: error: spec/routes: put is not a route verb: the partial update is patch, so write patch /z/:slug Users.Replace (§routes)",
		"10:15: error: spec/route-paths: write :slug, not :slug(string)",
	} {
		found := false
		for _, d := range diags {
			found = found || strings.Contains(d.String(), want)
		}
		if !found {
			t.Errorf("no diagnostic %q; got %v", want, diags)
		}
	}
	if len(diags) != 6 {
		t.Errorf("want the put error once per line beside the handler and path errors, 6 in all; got %v", diags)
	}
}

func TestVetUnusedPipeline(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod": modFile,
		"app/r.volt": `package app

pipeline api {
	use volt.RequestID
}

pipeline dead {
	use volt.Logger
}

scope / [pipe: api] {
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
			"go.mod":     modFile,
			"app/r.volt": "scope / {\n\tget /x X.Y\n}\n",
		}, "must begin with a package clause"},
		{"package name unlike its directory", map[string]string{
			"go.mod":          modFile,
			"lib__api/r.volt": "package api\n",
			"lib__api/a.go":   "package api\n",
			"app/r.volt":      "package routes\n",
			"app/a.go":        "package app\n",
		}, `package name "routes" disagrees with package app, which the directory's Go file a.go declares`},
		{"file name disagreement", map[string]string{
			"go.mod":     modFile,
			"app/a.volt": "package app\n",
			"app/b.volt": "package app2\n",
		}, "disagrees"},
		{"use is no element", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\nuse * from \"./x\"\n",
		}, `unknown element type "use"`},
		{"unknown import", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\nimport (\n\tnothere\n)\nscope / [pipe: p] { get /x X.Y }\n",
		}, "unknown package"},
		{"self import", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\nimport (\n\tapp\n)\n",
		}, "cannot import itself"},
		{"unused import", map[string]string{
			"go.mod":     modFile,
			"db/s.volt":  "package db\ntable users { id int [pk] }\n",
			"app/r.volt": "package app\nimport (\n\tdb\n)\n",
		}, "imported and not used"},
		{"import cycle", map[string]string{
			"go.mod":    modFile,
			"a/a.volt":  "package a\nimport (\n\tb\n)\nscope / { resources b.users }\n",
			"b/b.volt":  "package b\nimport (\n\ta\n)\nscope /b { resources a.users }\n",
			"a/t.volt":  "package a\ntable users { id int [pk] }\n",
			"b/t2.volt": "package b\ntable users { id int [pk] }\n",
		}, "import cycle"},
		{"duplicate route", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\nscope / {\n\tget /users/:id Users.Show\n\tget /users/:name Users.ByName\n}\n",
		}, "identical method and path shape"},
		{"helper collision", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\nscope / {\n\tget /a A.Show\n\tget /b B.Show [name: show]\n}\n",
		}, "already produced by the route"},
		{"signature conflict", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\nscope / {\n\tget /a/:id(int) X.Show\n\tget /b/:id X.Show\n}\n",
		}, "different signatures"},
		{"wildcard not last", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\nscope / {\n\tget /f/:p.../x F.S\n}\n",
		}, "must be the last path segment"},
		{"wildcard in scope prefix", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\nscope /f/:p... {\n\tget /x F.S\n}\n",
		}, "scope prefix cannot contain a wildcard"},
		{"unknown pipeline", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\nscope / [pipe: nope] {\n\tget /x X.Y\n}\n",
		}, "unknown pipeline"},
		{"unknown param type", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\nscope / {\n\tget /x/:id(uuid) X.Y\n}\n",
		}, "unknown parameter type"},
		{"duplicate param", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\nscope /u/:id {\n\tget /p/:id X.Y\n}\n",
		}, "duplicate path parameter"},
		{"unknown table", map[string]string{
			"go.mod":     modFile,
			"db/s.volt":  "package db\n\ntable users {\n\tid int [pk]\n}\n",
			"app/r.volt": "package app\nimport (\n\tdb\n)\nscope / {\n\tresources db.nope\n}\n",
		}, "no table"},
		{"table name case", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\n\ntable posts {\n\tid int [pk]\n}\n\nscope / {\n\tresources Posts\n}\n",
		}, "case-sensitive"},
		{"only and except", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\ntable users {\n\tid int [pk]\n}\nscope / {\n\tresources users [only: (index), except: (show)]\n}\n",
		}, "cannot be combined"},
		{"unknown action", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\ntable users {\n\tid int [pk]\n}\nscope / {\n\tresources users [only: (browse)]\n}\n",
		}, "unknown action"},
		{"lowercase controller", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\nscope / {\n\tget /x home.Index\n}\n",
		}, "exported Go identifiers"},
		{"scope setting typo", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\nscope / [pipes: api] {\n\tget /x X.Y\n}\n",
		}, "not valid on a scope"},
		{"keyword param", map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\nscope / {\n\tget /x/:type X.Y\n}\n",
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
	one, d1 := project(t, map[string]string{
		"go.mod": modFile,
		"app/all.volt": `package app
table users { id int [pk] }
pipeline api { use volt.RequestID }
scope / [pipe: api] {
	resources users
	get /about Home.About
}
`,
	})
	wantClean(t, d1)
	three, d3 := project(t, map[string]string{
		"go.mod":       modFile,
		"app/a_s.volt": "package app\ntable users { id int [pk] }\n",
		"app/b_p.volt": "package app\npipeline api { use volt.RequestID }\n",
		"app/c_r.volt": "package app\nscope / [pipe: api] {\n\tresources users\n\tget /about Home.About\n}\n",
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

func TestQueryRoutes(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod":         modFile,
		"db/schema.volt": "package db\n\ntable users {\n\tid int [pk]\n\temail string [unique]\n}\n\nselect picked for users where id in :ids and email like :pat\n",
		"app/r.volt": "package app\n\nimport (\n\tdb\n)\n\nscope /api [name: api] {\n" +
			"\tget /users/:id(int) db.UserGet\n\tpost /users db.UserCreate\n\tget /picked db.UserPicked\n\tdelete /users/:id(int) db.UserDelete\n}\n",
	})
	wantClean(t, diags)
	app := pr.Packages["app"]
	if len(app.Routes) != 4 || len(app.Controllers) != 0 {
		t.Fatalf("routes = %d, controllers = %d; want 4 and none", len(app.Routes), len(app.Controllers))
	}
	get := app.Routes[0].Query
	if get == nil || get.Import != "blog/db" || get.Result != "User" || get.Many || get.Status != 200 {
		t.Errorf("UserGet ref = %+v", get)
	}
	if len(get.Params) != 1 || get.Params[0].Source != FromPath || get.Params[0].GoType != "int" {
		t.Errorf("UserGet params = %+v, want id from the path as int", get.Params)
	}
	if app.Routes[0].HelperName != "APIUserGet" {
		t.Errorf("helper = %q, want APIUserGet", app.Routes[0].HelperName)
	}
	create := app.Routes[1].Query
	if create.Status != 201 || len(create.Params) != 1 || create.Params[0].Source != FromBody || create.Params[0].GoType != "db.UserChangeset" {
		t.Errorf("UserCreate ref = %+v", create)
	}
	if app.Routes[1].HelperName != "" {
		t.Errorf("a write has no helper, got %q", app.Routes[1].HelperName)
	}
	picked := app.Routes[2].Query
	if !picked.Many || len(picked.Params) != 2 || picked.Params[0].Source != FromList || picked.Params[0].GoType != "[]int" ||
		picked.Params[1].Source != FromQuery || picked.Params[1].GoType != "string" {
		t.Errorf("UserPicked ref = %+v", picked)
	}
	del := app.Routes[3].Query
	if del.Result != "" || del.Status != 204 {
		t.Errorf("UserDelete ref = %+v", del)
	}
}

func TestQueryRouteErrors(t *testing.T) {
	schema := "package db\n\ntable users {\n\tid int [pk]\n\temail string\n}\n\ntable notes {\n\tbody string\n}\n\nselect picked for users where id in :ids\n\ntable tags {\n\tslug string [pk]\n\tlabel string\n}\n"
	cases := []struct{ name, route, want string }{
		{"unknown", "get /x db.UserFetch", "no generated query db.UserFetch"},
		{"case hint", "get /x db.userget", `did you mean "UserGet"`},
		{"no pk", "get /x/:id(int) db.NoteGet", "no generated query db.NoteGet"},
		{"path type", "get /x/:id db.UserGet", "spell it :id(int)"},
		{"string annotation", "get /x/:id(string) db.UserGet", "write :id, not :id(string): a bare path parameter is a string"},
		{"list verb", "post /x db.UserList", "db.UserList is routed with get, not post: the generated method a query route binds fixes its verb (§query-routes)"},
		{"head verb", "head /x db.UserList", "db.UserList is routed with get, not head: the generated method a query route binds fixes its verb, and ServeMux answers HEAD on a get route"},
		{"head on a write", "head /x db.UserCreate", "db.UserCreate is routed with post, not head: the generated method a query route binds fixes its verb (§query-routes)"},
		{"any verb", "any /x db.UserList", "db.UserList is routed with get, not any"},
		{"get verb", "delete /x/:id(int) db.UserGet", "db.UserGet is routed with get, not delete"},
		{"select verb", "options /x db.UserPicked", "db.UserPicked is routed with get, not options"},
		{"create verb", "get /x db.UserCreate", "db.UserCreate is routed with post, not get"},
		{"update verb", "post /x/:id(int) db.UserUpdate", "db.UserUpdate is routed with patch, not post"},
		{"put update", "put /x/:id(int) db.UserUpdate", "put is not a route verb: the partial update is patch, so write patch /x/:id(int) db.UserUpdate (§routes)"},
		{"delete verb", "get /x/:id(int) db.UserDelete", "db.UserDelete is routed with delete, not get"},
		{"update key", "patch /x db.UserUpdate", `db.UserUpdate loads the row to update by "id", which must be a path parameter of the route; spell it :id(int)`},
		{"string update key", "patch /x db.TagUpdate", `db.TagUpdate loads the row to update by "slug", which must be a path parameter of the route; spell it :slug (§query-routes)`},
		{"string path type", "get /x/:slug(int) db.TagGet", "takes string; spell it :slug (§query-routes)"},
		{"update key elsewhere", "patch /x/:email db.UserUpdate", `path parameter "email" is not a parameter of db.UserUpdate`},
		{"extra path param", "get /x/:n(int) db.UserList", `path parameter "n" is not a parameter of db.UserList`},
		{"list in path", "get /x/:ids db.UserPicked", "cannot be a path parameter"},
		{"wildcard", "get /x/:id... db.UserGet", "cannot be a wildcard"},
		{"not a data package", "get /x app2.Thing", "declares no tables"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{
				"go.mod":         modFile,
				"db/schema.volt": schema,
				"app2/x.volt":    "package app2\n\nscope /z {\n\tget /q Q.R\n}\n",
				"app/r.volt":     "package app\n\nimport (\n\tdb\n\tapp2\n)\n\nscope / {\n\tget /keep app2.Z\n\t" + tc.route + "\n}\n",
			}
			if tc.name != "not a data package" {
				files["app/r.volt"] = "package app\n\nimport (\n\tdb\n)\n\nscope / {\n\t" + tc.route + "\n}\n"
			}
			_, diags := project(t, files)
			wantError(t, diags, tc.want)
		})
	}
}

func TestDatasetExpansion(t *testing.T) {
	schema := "package db\n\ntable ms_revenue {\n\tid int [pk]\n\tyear int\n}\n\ntable ms_usage {\n\tid int [pk]\n\tyear int\n}\n\ntable ms_notes {\n\tid int [pk]\n\tyear int\n}\n\ngroup series {\n\tms_revenue\n\tms_usage\n\tms_notes\n}\n\nselect browse for series where year = :year\n"
	pr, diags := project(t, map[string]string{
		"go.mod":         modFile,
		"db/schema.volt": schema,
		"app/r.volt":     "package app\n\nimport (\n\tdb\n)\n\nscope /ms [name: ms] {\n\tdataset db.browse [strip: \"ms_\", except: (ms_notes)]\n}\n",
	})
	wantClean(t, diags)
	routes := pr.Packages["app"].Routes
	if len(routes) != 2 {
		t.Fatalf("routes = %d, want 2: %v", len(routes), routes)
	}
	if routes[0].Pattern != "/ms/revenue" || routes[0].Query == nil || routes[0].Query.Method != "MsRevenueBrowse" ||
		routes[0].HelperName != "MsMsRevenueBrowse" || !routes[0].FromDataset {
		t.Errorf("first route = %+v", routes[0])
	}
	if routes[1].Pattern != "/ms/usage" || routes[1].Query.Method != "MsUsageBrowse" {
		t.Errorf("second route = %+v", routes[1])
	}
	if p := routes[0].Query.Params; len(p) != 1 || p[0].Source != FromQuery || p[0].GoType != "int" {
		t.Errorf("params = %+v", p)
	}

	for _, tc := range []struct{ name, line, want string }{
		{"only keeps", "dataset db.browse [only: (ms_usage)]", ""},
		{"strip miss", "dataset db.browse [strip: \"x_\"]", "is not a prefix"},
		{"both lists", "dataset db.browse [only: (ms_usage), except: (ms_notes)]", "cannot both be set"},
		{"unknown select", "dataset db.Browse", `did you mean "browse"`},
		{"bad setting", "dataset db.browse [api]", "not valid on a dataset"},
		{"collides with a route", "dataset db.browse\n\tget /ms_usage Ms.Usage", "conflicts with the route"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pr, diags := project(t, map[string]string{
				"go.mod":         modFile,
				"db/schema.volt": schema,
				"app/r.volt":     "package app\n\nimport (\n\tdb\n)\n\nscope /ms {\n\t" + tc.line + "\n}\n",
			})
			if tc.want == "" {
				wantClean(t, diags)
				if n := len(pr.Packages["app"].Routes); n != 1 {
					t.Errorf("only: routes = %d, want 1", n)
				}
				return
			}
			wantError(t, diags, tc.want)
		})
	}
}

func TestEventRoute(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod":     modFile,
		"app/r.volt": "package app\n\nscope /live [name: live] {\n\tget /events volt.Events\n}\n",
	})
	wantClean(t, diags)
	app := pr.Packages["app"]
	if len(app.Routes) != 1 || len(app.Controllers) != 0 {
		t.Fatalf("routes = %d, controllers = %d; want 1 and none", len(app.Routes), len(app.Controllers))
	}
	r := app.Routes[0]
	if !r.Events || r.HelperName != "LiveEvents" || r.ClientName != "LiveEvents" || r.HandlerRef() != "volt.Events" {
		t.Errorf("event route = %+v", r)
	}
	for _, tc := range []struct{ name, route, want string }{
		{"unknown runtime handler", "get /events volt.Stream", "provides no handler volt.Stream"},
		{"wrong verb", "post /events volt.Events", "read with get"},
	} {
		_, diags := project(t, map[string]string{
			"go.mod":     modFile,
			"app/r.volt": "package app\n\nscope / {\n\t" + tc.route + "\n}\n",
		})
		wantError(t, diags, tc.want)
	}
}

func TestTypeGoDeclarations(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod": modFile,
		"db/schema.volt": `package db

type State    string
type Level    int
type Rank     int
type Flag     bool
type Box      string
type Ghost    string
type Tested   string
type Blob     []byte
type Stamp    time.Time
type kinds.Kind string from "example.com/kinds"

table items {
  id     int    [pk]
  state  State
  level  Level
  rank   Rank
  flag   Flag
  box    Box
  ghost  Ghost
  tested Tested
  blob   Blob
  stamp  Stamp
  kind   kinds.Kind
}
`,
		"db/kinds.go":      "package db\n\nimport \"time\"\n\ntype State string\n\ntype Level = int\n\ntype Rank int64\n\ntype Flag bool\n\ntype Box[T any] string\n\ntype Blob []byte\n\ntype Stamp time.Time\n",
		"db/nao_extra.go":  "// Code generated by hand for this test; DO NOT EDIT.\n\npackage db\n\ntype Ghost string\n",
		"db/kinds_test.go": "package db\n\ntype Tested string\n",
	})
	kinded := map[string]bool{}
	for _, td := range pr.Packages["db"].Schema().Types {
		kinded[td.Key] = td.Kinded
	}
	want := map[string]bool{"State": true, "Level": true, "Rank": false, "Flag": true, "Box": false, "Ghost": false, "Tested": false, "Blob": false, "Stamp": false, "kinds.Kind": false}
	for name, w := range want {
		if kinded[name] != w {
			t.Errorf("type %s kinded = %v, want %v (§types)", name, kinded[name], w)
		}
	}
	var missing []string
	for _, d := range diags {
		if strings.Contains(d.Msg, "is declared in no Go file") {
			missing = append(missing, d.Msg)
		}
	}
	if len(missing) != 2 || !strings.Contains(missing[0], "type Ghost ") || !strings.Contains(missing[1], "type Tested ") {
		t.Errorf("a type declared only in a generated or a test file is missing, and only those two: %v", missing)
	}

	_, diags = project(t, map[string]string{
		"go.mod":         modFile,
		"db/schema.volt": "package db\n\ntype State string\n\ntable items {\n  id    int   [pk]\n  state State\n}\n",
		"db/broken.go":   "package db\n\nfunc half( {\n\ntype State string\n",
	})
	wantError(t, diags, "type State is declared in no Go file of package db; write type State string beside the schema (note: broken.go: ")
	for _, d := range diags {
		if strings.Contains(d.Msg, "type State") && (!strings.Contains(d.Msg, "declarations after a syntax error are invisible") || !strings.HasSuffix(d.Msg, "(§types)")) {
			t.Errorf("the note names the broken file and the message ends with its section: %s", d.Msg)
		}
	}
}

func TestRouteLoads(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod":         modFile,
		"db/schema.volt": "package db\n\ntable metrics {\n\tid   int [pk]\n\tunit string\n}\n\nselect ordered for metrics [order: (id asc)]\nselect by_unit for metrics where unit = :unit and id in :ids\n",
		"app/r.volt": "package app\n\nimport (\n\tdb\n)\n\nscope / {\n" +
			"\tget /metrics/:id(int) Metrics.Show [timeout: 5] {\n\t\tmetric db.MetricGet(id)\n\t\tpeers  db.MetricByUnit(\"ms\", ids)\n\t\tall    db.MetricOrdered()\n\t}\n}\n",
	})
	wantClean(t, diags)
	app := pr.Packages["app"]
	if len(app.Routes) != 1 || len(app.Controllers) != 1 {
		t.Fatalf("routes = %d, controllers = %d; want 1 and 1", len(app.Routes), len(app.Controllers))
	}
	r := app.Routes[0]
	if r.Timeout != 5 || len(r.Loads) != 3 {
		t.Fatalf("route = %+v; want a timeout of 5 and three loads", r)
	}
	metric, peers, all := r.Loads[0], r.Loads[1], r.Loads[2]
	if metric.Name != "metric" || metric.GoType() != "db.Metric" || len(metric.Query.Params) != 1 || metric.Query.Params[0].Source != FromPath {
		t.Errorf("metric load = %+v %+v", metric, metric.Query)
	}
	if peers.GoType() != "[]db.Metric" || len(peers.Query.Params) != 2 || peers.Query.Params[0].Source != FromLiteral || peers.Query.Params[0].Literal != `"ms"` ||
		peers.Query.Params[1].Source != FromList || peers.Query.Params[1].GoType != "[]int" {
		t.Errorf("peers load = %+v", peers.Query.Params)
	}
	if all.GoType() != "[]db.Metric" || len(all.Query.Params) != 0 {
		t.Errorf("all load = %+v", all.Query)
	}
	action := app.Controllers["Metrics"].Actions[0]
	if action.Name != "Show" || len(action.Loads) != 3 || action.Loads[0].Name != "metric" {
		t.Errorf("action = %+v; want Show with the three loads in order", action)
	}
}

// TestRouteLoadNumberLiterals: a number a load or a body key binds is
// spelled in canonical Go form (§route-loads, §predicates), so a leading
// zero never makes Go read an octal literal, or one it cannot compile.
func TestRouteLoadNumberLiterals(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod":         modFile,
		"db/schema.volt": "package db\n\ntable metrics {\n\tid    int [pk]\n\tscore float64\n}\n\nselect above for metrics where id > :id and score > :score [order: (id asc), limit: :limit]\n",
		"app/r.volt": "package app\n\nimport (\n\tdb\n)\n\nscope / {\n" +
			"\tget /metrics Metrics.Index {\n\t\toctal db.MetricAbove(010, 2.50, 08)\n\t\tnegative db.MetricAbove(-010, 1e2, 1)\n\t}\n" +
			"\tpatch /metrics/first Metrics.Save [body: db.MetricUpdate(010)]\n}\n",
	})
	wantClean(t, diags)
	app := pr.Packages["app"]
	literals := func(q *QueryRef) []string {
		var out []string
		for _, p := range q.Params {
			out = append(out, p.Literal)
		}
		return out
	}
	r := app.Routes[0]
	for i, want := range [][]string{{"10", "2.5", "8"}, {"-10", "100.0", "1"}} {
		if got := literals(r.Loads[i].Query); !slices.Equal(got, want) {
			t.Errorf("load %s binds %q; want %q", r.Loads[i].Name, got, want)
		}
	}
	if got := literals(app.Routes[1].Body.Query); !slices.Equal(got, []string{"10"}) {
		t.Errorf("body key binds %q; want [10]", got)
	}
}

func TestRequestBodies(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod":         modFile,
		"db/schema.volt": "package db\n\ntable metrics {\n\tid   int [pk]\n\tunit string\n\tnote string\n}\n",
		"app/forms.go":   "package app\n\ntype LoginForm struct {\n\tEmail string\n}\n",
		"app/r.volt": "package app\n\nimport (\n\tdb\n)\n\nscope / {\n" +
			"\tpatch /metrics/:id(int) Metrics.Save [body: db.MetricUpdate(id), only: (unit)]\n" +
			"\tpost  /metrics          Metrics.Create [body: db.MetricCreate]\n" +
			"\tpost  /login            Session.Login [body: LoginForm]\n}\n",
	})
	wantClean(t, diags)
	app := pr.Packages["app"]
	if len(app.Routes) != 3 {
		t.Fatalf("routes = %d; want 3", len(app.Routes))
	}
	save, create, login := app.Routes[0].Body, app.Routes[1].Body, app.Routes[2].Body
	if save == nil || save.Kind != BodyChangeset || save.GoType() != "db.MetricChangeset" || save.Query.Changeset.Get != "MetricGet" ||
		len(save.Query.Params) != 1 || save.Query.Params[0].Source != FromPath || save.Query.Params[0].Name != "id" {
		t.Errorf("save body = %+v %+v", save, save.Query)
	}
	if got := save.Query.Changeset.Allow; len(got) != 1 || got[0] != "MetricUnit" {
		t.Errorf("only: narrowed the allow list to %v; want [MetricUnit]", got)
	}
	if create == nil || create.GoType() != "db.MetricChangeset" || create.Query.Changeset.New != "MetricNew" || len(create.Query.Params) != 0 {
		t.Errorf("create body = %+v %+v", create, create.Query)
	}
	if got := create.Query.Changeset.Allow; len(got) != 2 {
		t.Errorf("create allow list = %v; want the write's own", got)
	}
	if login == nil || login.Kind != BodyStruct || login.GoType() != "LoginForm" {
		t.Errorf("login body = %+v", login)
	}
	if a := app.Controllers["Metrics"].Action("Save"); a == nil || a.Body != save {
		t.Errorf("Save action = %+v; want the route's body", a)
	}
}

func TestSelectJoins(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod": modFile,
		"db/schema.volt": "package db\n\ntable metrics {\n\tid   int [pk]\n\tname string\n}\n\n" +
			"table points {\n\tid        int [pk]\n\tmetric_id int [ref: metrics.id]\n\tsource_id int [null]\n\tts        int\n}\n\n" +
			"ref: points.source_id > metrics.id [as: source, reverse: sourced]\n\n" +
			"select recent (id, ts, metric.name, source.name) for points join metric join source where ts >= :since and metric.name = :name [order: (metric.name asc, ts desc)]\n" +
			"select series (id, points.ts) for metrics join points\n",
	})
	wantClean(t, diags)
	db := pr.Packages["db"]
	if len(db.Selects) != 2 {
		t.Fatalf("selects = %d; want 2", len(db.Selects))
	}
	recent, series := db.Selects[0], db.Selects[1]
	if len(recent.Joins) != 2 || recent.Joins[0].Name != "metric" || recent.Joins[0].Left || recent.Joins[1].Name != "source" || !recent.Joins[1].Left {
		t.Errorf("recent joins = %+v; want metric inner and source left", recent.Joins)
	}
	var cols []string
	for _, it := range recent.Items {
		cols = append(cols, it.Col)
	}
	if got := strings.Join(cols, ","); got != "id,ts,metric.name,source.name" {
		t.Errorf("recent cols = %s", got)
	}
	if len(series.Joins) != 1 || !series.Joins[0].Left || series.Joins[0].Table.Key != "points" {
		t.Errorf("series joins = %+v; want the reverse relation, left", series.Joins)
	}
	plan := db.Plan()
	fns := db.SelectFns()
	if len(fns) != 2 {
		t.Fatalf("select functions = %d; want 2", len(fns))
	}
	sql, args, err := plan.SelectStatement(fns[0])
	if err != nil {
		t.Fatal(err)
	}
	want := `SELECT "points"."id", "points"."ts", "metric"."name", "source"."name" FROM "points" INNER JOIN "metrics" AS "metric" ON "metric"."id" = "points"."metric_id" LEFT JOIN "metrics" AS "source" ON "source"."id" = "points"."source_id" WHERE ("points"."ts" >= ? AND "metric"."name" = ?) ORDER BY "metric"."name" ASC, "points"."ts" DESC`
	if sql != want || strings.Join(args, ",") != "since,name" {
		t.Errorf("recent renders\n\t%s %v\nwant\n\t%s [since name]", sql, args, want)
	}
	row, fields, err := plan.SelectRowType(fns[0])
	if err != nil {
		t.Fatal(err)
	}
	var sigs []string
	for _, f := range fields {
		sigs = append(sigs, f.Name+" "+f.Type+" "+f.Col)
	}
	if row != "Recent" || strings.Join(sigs, "; ") != "ID int id; Ts int ts; MetricName string name; SourceName rt.Null[string] name" {
		t.Errorf("recent row = %s %v", row, sigs)
	}
	sql, _, err = plan.SelectStatement(fns[1])
	if err != nil {
		t.Fatal(err)
	}
	if want := `SELECT "metrics"."id", "points"."ts" FROM "metrics" LEFT JOIN "points" AS "points" ON "points"."metric_id" = "metrics"."id"`; sql != want {
		t.Errorf("series renders %s, want %s", sql, want)
	}
}

func TestSelectAggregates(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod": modFile,
		"db/schema.volt": "package db\n\ntable points {\n\tid    int [pk]\n\tsite  string\n\tts    int\n\tvalue float64\n\tnote  string [null]\n}\n\n" +
			"select hourly (site, ts / 3600 * 3600 as hour, avg(value) as mean, count() as n, max(note) as last) for points where ts >= :since [group: (site, hour), order: (hour asc, n desc)]\n" +
			"select totals (count() as n, sum(value) as total) for points\n" +
			"select scaled (id, value * :factor + 1 as scaled, note as memo) for points\n",
	})
	wantClean(t, diags)
	db := pr.Packages["db"]
	if len(db.Selects) != 3 {
		t.Fatalf("selects = %d; want 3", len(db.Selects))
	}
	hourly, totals, scaled := db.Selects[0], db.Selects[1], db.Selects[2]
	if hourly.Lookup || !totals.Lookup || scaled.Lookup {
		t.Errorf("lookups: hourly %v, totals %v, scaled %v; want only totals", hourly.Lookup, totals.Lookup, scaled.Lookup)
	}
	if got := strings.Join(hourly.Group, ","); got != "site,hour" {
		t.Errorf("hourly group = %s", got)
	}
	types := func(si *SelectInfo) string {
		var out []string
		for _, it := range si.Items {
			out = append(out, it.Alias+":"+it.GoType)
		}
		return strings.Join(out, " ")
	}
	if got := types(hourly); got != ": hour:int mean:float64 n:int last:rt.Null[string]" {
		t.Errorf("hourly items = %q", got)
	}
	if got := types(totals); got != "n:int total:rt.Null[float64]" {
		t.Errorf("totals items = %q", got)
	}
	if got := types(scaled); got != ": scaled:float64 memo:rt.Null[string]" {
		t.Errorf("scaled items = %q", got)
	}
	plan, fns := db.Plan(), db.SelectFns()
	sql, args, err := plan.SelectStatement(fns[0])
	if err != nil {
		t.Fatal(err)
	}
	want := `SELECT "site", (("ts" / ?) * ?) AS "hour", avg("value") AS "mean", count(*) AS "n", max("note") AS "last" FROM "points" WHERE "ts" >= ? GROUP BY "site", "hour" ORDER BY "hour" ASC, "n" DESC`
	if sql != want || strings.Join(args, ",") != "3600,3600,since" {
		t.Errorf("hourly renders\n\t%s %v\nwant\n\t%s [3600 3600 since]", sql, args, want)
	}
	if sql, args, err = plan.SelectStatement(fns[2]); err != nil || sql != `SELECT "id", (("value" * ?) + ?) AS "scaled", "note" AS "memo" FROM "points"` || strings.Join(args, ",") != "factor,1.0" {
		t.Errorf("scaled renders %s %v, %v", sql, args, err)
	}
}

func TestSelectWith(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod": modFile,
		"db/schema.volt": "package db\n\ntable metrics {\n\tid   int [pk]\n\tname string\n}\n\n" +
			"table points {\n\tid        int [pk]\n\tmetric_id int [ref: metrics.id]\n\tts        int\n}\n\n" +
			"select board for metrics with points where name = :name [order: (id asc)]\n",
	})
	wantClean(t, diags)
	db := pr.Packages["db"]
	if len(db.Selects) != 1 || db.Selects[0].With == nil {
		t.Fatalf("selects = %+v; want one nested select", db.Selects)
	}
	w := db.Selects[0].With
	if w.Name != "points" || w.Table.Key != "points" || w.ParentKey != "id" || w.ChildKey != "metric_id" || w.Field != "Points" || w.RowType != "MetricWithPoints" {
		t.Errorf("with = %+v", w)
	}
	fns := db.SelectFns()
	child, err := db.Plan().WithChildSQL(fns[0])
	if err != nil {
		t.Fatal(err)
	}
	if want := `SELECT "id", "metric_id", "ts" FROM "points" WHERE "metric_id" IN (SELECT value FROM json_each(?)) ORDER BY "id" ASC`; child != want {
		t.Errorf("children render %s, want %s", child, want)
	}
}

func TestScopeLoads(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod":         modFile,
		"db/schema.volt": "package db\n\ntable users {\n\tid int [pk]\n}\n",
		"app/loads.go": "package app\n\nimport (\n\t\"github.com/Piechutowski/volt\"\n\t\"blog/db\"\n)\n\n" +
			"func CurrentUser(r volt.Request) volt.Load[db.User] { return volt.LoadWith(db.UserGet(1)) }\n" +
			"func Tenant(r volt.Request) volt.Load[string] { return volt.Halt[string](volt.NoContent()) }\n",
		"app/r.volt": "package app\n\nimport (\n\tdb\n)\n\nscope / {\n\tload user CurrentUser\n\tget /me Me.Show\n" +
			"\tscope /t {\n\t\tload tenant Tenant\n\t\tget /stats Admin.Stats\n\t\tget /users db.UserList\n\t}\n}\n",
	})
	wantClean(t, diags)
	app := pr.Packages["app"]
	if len(app.Routes) != 3 {
		t.Fatalf("routes = %d; want 3", len(app.Routes))
	}
	show, stats, list := app.Routes[0], app.Routes[1], app.Routes[2]
	if len(show.ScopeLoads) != 1 || show.ScopeLoads[0].Name != "user" || show.ScopeLoads[0].Func != "CurrentUser" || show.ScopeLoads[0].GoType != "db.User" ||
		show.ScopeLoads[0].Qualifier != "db" || show.ScopeLoads[0].Import != "blog/db" || show.ScopeLoads[0].PkgName != "db" {
		t.Errorf("show loads = %+v", show.ScopeLoads)
	}
	if len(stats.ScopeLoads) != 2 || stats.ScopeLoads[1].Name != "tenant" || stats.ScopeLoads[1].GoType != "string" || stats.ScopeLoads[1].Import != "" {
		t.Errorf("stats loads = %+v; want user then tenant", stats.ScopeLoads)
	}
	if len(list.ScopeLoads) != 2 || list.Query == nil {
		t.Errorf("list = %+v; want a query route under both loads", list)
	}
	if a := app.Controllers["Admin"].Action("Stats"); a == nil || len(a.ScopeLoads) != 2 || len(a.Params) != 0 {
		t.Errorf("Stats action = %+v; want two scope loads and no parameter", a)
	}
}

func TestQueryRoutesOverDerivedSelects(t *testing.T) {
	pr, diags := project(t, map[string]string{
		"go.mod": modFile,
		"db/schema.volt": "package db\n\ntable metrics {\n\tid int [pk]\n}\n\ntable points {\n\tid        int [pk]\n\tmetric_id int [ref: metrics.id]\n}\n\n" +
			"select totals (count() as n) for points\nselect board for metrics with points\n",
		"app/r.volt": "package app\n\nimport (\n\tdb\n)\n\nscope / {\n\tget /totals db.PointTotals\n\tget /board db.MetricBoard\n" +
			"\tget /stats Pages.Stats {\n\t\ttotals db.PointTotals()\n\t\tboard db.MetricBoard()\n\t}\n}\n",
	})
	wantClean(t, diags)
	app := pr.Packages["app"]
	if len(app.Routes) != 3 {
		t.Fatalf("routes = %d; want 3", len(app.Routes))
	}
	if q := app.Routes[0].Query; q.Many || q.Result != "Totals" {
		t.Errorf("totals route = %+v; want one Totals row", q)
	}
	if q := app.Routes[1].Query; !q.Many || q.Result != "MetricWithPoints" {
		t.Errorf("board route = %+v; want many MetricWithPoints rows", q)
	}
	loads := app.Routes[2].Loads
	if len(loads) != 2 || loads[0].GoType() != "db.Totals" || loads[1].GoType() != "[]db.MetricWithPoints" {
		t.Errorf("stats loads = %+v", loads)
	}
}
