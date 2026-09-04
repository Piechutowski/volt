package lang

import (
	"github.com/Piechutowski/volt/lang/token"
)

// ParamType is the closed set of route parameter types (spec §V4.1.3).
// The names are DBML-facing; GoType maps them to Go. The set matches the
// Go types nao's model generator emits for primary keys, so a
// model-inferred parameter always agrees with the model field it joins.
type ParamType string

const (
	TInt    ParamType = "int"
	TInt32  ParamType = "int32"
	TInt64  ParamType = "int64"
	TString ParamType = "string"
)

// KnownParamType reports whether s names a parameter type.
func KnownParamType(s string) bool {
	switch ParamType(s) {
	case TInt, TInt32, TInt64, TString:
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
	Type   ParamType // TString for wildcards
	Wild   bool      // rest-of-path capture (":name...")
}

// RouteInfo is one fully expanded route: scope prefixes applied,
// resources unrolled, pipelines and error handler resolved.
type RouteInfo struct {
	Method  string // "GET", ... ; "" for the any-verb (matches every method)
	Pattern string // ServeMux registration pattern, e.g. "/users/{id}"
	Spelled string // canonical DSL spelling, e.g. "/users/:id(int64)"
	Params  []Param

	Controller string // Go interface base name, e.g. "Users"
	Action     string // method name, e.g. "Show"

	HelperName   string // reverse-URL function base name; "" when suppressed
	ClientName   string // generated client method name (§V4.10): every query route, and every controller route with a helper
	Pipes        []string
	ErrorHandler string // package-level function name; "" for the default

	// Query is set for a query route (§V4.8): the handler is a generated
	// query method of an imported data package, and Controller/Action
	// are empty.
	Query *QueryRef

	Pos           token.Position
	FromResources bool
}

// QuerySource says where a query route binds one parameter from.
type QuerySource int

const (
	FromPath  QuerySource = iota // a typed path parameter of the route
	FromQuery                    // one query-string key, parsed to the type
	FromList                     // a repeated query-string key (list parameter)
	FromBody                     // the request body, decoded by Content-Type
)

// QueryParam is one parameter of a query route's method, in signature
// order after ctx.
type QueryParam struct {
	Name   string // Go parameter name, also the URL key
	GoType string // unqualified Go type ("int32", "[]string"); params structs are qualified ("db.UserCreateParams")
	Source QuerySource
}

// QueryRef binds a route to a generated query method of an imported
// data package (§V4.8).
type QueryRef struct {
	Qualifier string // the import qualifier as written, e.g. "db"
	Field     string // the Controllers field holding *pkg.Queries: the qualifier as a Go name, e.g. "DB"
	Package   string // root-relative package path
	Import    string // Go import path of the package
	PkgName   string // the package's Go name (its directory name)
	Method    string // generated method name, e.g. "UserGet"
	Params    []QueryParam
	Result    string // row type, unqualified ("User"); "" for delete
	Many      bool   // slice result
	Status    int    // success status: 200, 201 for create, 204 for delete
}

// Ref renders the reference as written, e.g. "db.UserGet".
func (q *QueryRef) Ref() string { return q.Qualifier + "." + q.Method }

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
// Two routes may share an action only with identical signatures (§V4.3).
type ActionInfo struct {
	Name   string
	Params []Param
	Routes []*RouteInfo
}

// resourceActions is the canonical action table (spec §V5.2), in
// definition order. Method "" is filled per action; Update expands to
// both PATCH and PUT.
var resourceActions = []struct {
	Name    string   // action / interface method name
	Methods []string // HTTP methods
	Suffix  string   // path suffix after /<table>
	OnID    bool     // route includes the key parameter
	API     bool     // included under [api]
}{
	{"Index", []string{"GET"}, "", false, true},
	{"New", []string{"GET"}, "/new", false, false},
	{"Create", []string{"POST"}, "", false, true},
	{"Show", []string{"GET"}, "", true, true},
	{"Edit", []string{"GET"}, "/edit", true, false},
	{"Update", []string{"PATCH", "PUT"}, "", true, true},
	{"Delete", []string{"DELETE"}, "", true, true},
}

// actionByLower maps the lowercased action names accepted in only:/except:
// lists to canonical names.
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
