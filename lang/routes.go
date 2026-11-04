package lang

import (
	"strings"

	"github.com/Piechutowski/volt/lang/token"
)

// ParamType is the closed set of route parameter types (spec §route-paths);
// GoType maps them to Go. The set matches the Go types nao's model
// generator emits for primary keys.
type ParamType string

const (
	TInt    ParamType = "int"
	TString ParamType = "string"

	tUnresolved ParamType = ""
)

// KnownParamType reports whether s names a parameter type.
func KnownParamType(s string) bool {
	switch ParamType(s) {
	case TInt, TString:
		return true
	}
	return false
}

// GoType returns the Go spelling of the parameter type.
func (t ParamType) GoType() string { return string(t) }

// Param is one captured path parameter of a route.
type Param struct {
	Name   string    // as written in the path (Go-visible name is derived)
	GoName string    // parameter name in generated signatures
	Type   ParamType // TString for wildcards; "" for a resources key whose column type is unknown (§columns)
	Wild   bool      // rest-of-path capture (":name...")
}

// RouteInfo is one fully expanded route: scope prefixes applied,
// resources unrolled, pipelines and error handler resolved.
type RouteInfo struct {
	Method  string // "GET", ... ; "" for the any-verb (matches every method)
	Pattern string // ServeMux registration pattern, e.g. "/users/{id}"
	Spelled string // canonical DSL spelling, e.g. "/users/:id(int)"
	Params  []Param

	Controller string // Go interface base name, e.g. "Users"
	Action     string // method name, e.g. "Show"

	HelperName   string // reverse-URL function base name; "" when suppressed
	ClientName   string // generated client method name (§generated-client): every query route, and every controller route with a helper
	Pipes        []string
	ErrorHandler string // package-level function name; "" for the default

	// Events marks an event route (`get /events volt.Events`, §event-routes):
	// the runtime's broker serves it; Controller/Action read "volt"/"Events".
	Events bool

	// Query is set for a query route (§query-routes): the handler is a generated
	// query method of an imported data package, and Controller/Action
	// are empty.
	Query *QueryRef

	// ScopeLoads are the loads of the enclosing scopes, outermost first
	// (§scope-loads), which the handler receives before its parameters.
	ScopeLoads []ScopeLoad
	// Loads are the route's loads in declaration order (§route-loads), and
	// Timeout the seconds of its timeout: setting, 0 when none.
	Loads   []LoadInfo
	Timeout int
	// Body is how a controller route binds its request body, nil when it
	// takes none (§request-bodies).
	Body *BodyInfo

	Pos           token.Position
	FromResources bool
	Table         string // the resources declaration's table name, when FromResources
	FromDataset   bool   // expanded from a dataset item (§datasets)

	collection bool
	shape      pathShape
	shapeKey   string
	ord        int
}

// QuerySource says where a query route binds one parameter from.
type QuerySource int

const (
	FromPath    QuerySource = iota // a typed path parameter of the route
	FromQuery                      // one query-string key, parsed to the type
	FromList                       // a repeated query-string key (list parameter)
	FromBody                       // the request body, decoded by Content-Type
	FromLiteral                    // a literal written in a load (§route-loads)
)

// QueryParam is one parameter of a query function, in signature order.
type QueryParam struct {
	Name    string // Go parameter name, also the URL key
	GoType  string // unqualified Go type ("int", "[]string"); a changeset is qualified ("db.UserChangeset")
	Source  QuerySource
	Literal string // the Go spelling of a FromLiteral argument (§route-loads)
	Limit   bool   // the select's limit: parameter, which a request value binds only when at least 1 (§selects-over-groups)
}

// LoadInfo is one load of a controller route (§route-loads): the name the
// handler receives the value under and the query it runs, its parameters
// bound from the route, the query string or literals.
type LoadInfo struct {
	Name  string
	Query *QueryRef
}

// GoType is the Go type of the loaded value as the routes package spells
// it: the row type, qualified unless the query is the package's own, a
// slice of it for a query of many rows.
func (ld LoadInfo) GoType() string {
	t := ld.Query.Result
	if !ld.Query.Local {
		t = ld.Query.Qualifier + "." + t
	}
	if ld.Query.Many {
		t = "[]" + t
	}
	return t
}

// ChangesetBind is how a query route builds the changeset of a write
// (spec §query-routes, §changesets): the body decodes as Params and is
// cast into New() for a create, or into the Changeset of the row Get
// loads by the key for an update, allowing the Allow columns. The key
// is the FromPath parameters that precede the body in Params; every
// key column of an update is one. The names are unqualified.
type ChangesetBind struct {
	Type   string   // changeset type, e.g. "UserChangeset"
	Params string   // write body type, e.g. "UserParams"
	New    string   // constructor of a create, e.g. "UserNew"; "" for an update
	Get    string   // row loader of an update, e.g. "UserGet"; "" for a create
	Allow  []string // column handles the cast allows, e.g. "UserEmail"
	Table  string   // the table's key in its package, which only: of a body reads (§request-bodies)
}

// QueryRef binds a route to a generated query method of a data package
// (§query-routes): an imported one, or the routing package itself when it
// declares the tables (Local).
type QueryRef struct {
	Qualifier string // the import qualifier as written, e.g. "db"; the package's own name when Local
	Package   string // root-relative package path
	Import    string // Go import path of the package
	PkgName   string // the package's Go name (its directory name)
	Local     bool   // the query is this package's own (§query-routes): no import, the call is unqualified
	Method    string // generated method name, e.g. "UserGet"
	Params    []QueryParam
	Changeset *ChangesetBind // how a write's body becomes its changeset (§changesets); nil for a read
	Result    string         // row type, unqualified ("User"); "" for delete
	Many      bool           // slice result
	Status    int            // success status: 200, 201 for create, 204 for delete

	Unresolved bool // the key's column type is unknown (§columns): checked as usual, Params lacks the key, never generated
}

// Ref renders the reference as written, e.g. "db.UserGet"; a query of
// the package itself is unqualified ("UserGet").
func (q *QueryRef) Ref() string {
	if q.Local {
		return q.Method
	}
	return q.Qualifier + "." + q.Method
}

// ControllerInfo groups the actions dispatched to one controller
// interface, for the generator.
type ControllerInfo struct {
	Name    string
	Actions []*ActionInfo
}

// Action returns the named action, or nil.
func (c *ControllerInfo) Action(name string) *ActionInfo {
	for _, a := range c.Actions {
		if a.Name == name {
			return a
		}
	}
	return nil
}

// ActionInfo is one interface method: its name and parameter signature.
// Two routes may share an action only with identical signatures (§handler-references).
type ActionInfo struct {
	Name       string
	Params     []Param
	Body       *BodyInfo   // the body the action receives after its parameters, nil when none (§request-bodies)
	ScopeLoads []ScopeLoad // the scope loads the action receives first (§scope-loads)
	Loads      []LoadInfo  // the loaded values the action receives after the body (§route-loads)
	Routes     []*RouteInfo
}

var resourceActions = []struct {
	Name   string
	Method string
	Suffix string
	OnID   bool
	API    bool
	Op     string
}{
	{"Index", "GET", "", false, true, "list"},
	{"New", "GET", "/new", false, false, ""},
	{"Create", "POST", "", false, true, "create"},
	{"Show", "GET", "", true, true, "get"},
	{"Edit", "GET", "/edit", true, false, ""},
	{"Update", "PATCH", "", true, true, "update"},
	{"Delete", "DELETE", "", true, true, "delete"},
}

var queryMethods = map[string]string{
	"select": "GET",
	"list":   "GET",
	"get":    "GET",
	"create": "POST",
	"update": "PATCH",
	"delete": "DELETE",
}

var actionByLower = func() map[string]string {
	m := map[string]string{}
	for _, a := range resourceActions {
		m[lowerASCII(a.Name)] = a.Name
	}
	return m
}()

func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// ScopeLoad is one scope load as a route beneath it sees it (§scope-loads):
// the parameter's name, the function of the routes package that
// decides it, the Go type it loads and, for a type of an imported
// package, the qualifier and the import.
type ScopeLoad struct {
	Name      string
	Func      string
	GoType    string
	Qualifier string // the import qualifier GoType names, "" for a type of this package
	Import    string // the Go import path behind Qualifier
	PkgName   string // the imported package's Go name
	Pos       token.Position
}

func scopeLoadsEqual(a, b []ScopeLoad) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].GoType != b[i].GoType {
			return false
		}
	}
	return true
}

func scopeLoadsString(ls []ScopeLoad) string {
	if len(ls) == 0 {
		return ""
	}
	parts := make([]string, len(ls))
	for i, ld := range ls {
		parts[i] = ld.Name + " " + ld.GoType
	}
	return "<" + strings.Join(parts, ", ") + "> "
}
