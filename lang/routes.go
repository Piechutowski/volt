package lang

import (
	"github.com/Piechutowski/volt/nao/edbml/token"
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
	Pipes        []string
	ErrorHandler string // package-level function name; "" for the default

	Pos           token.Position
	FromResources bool
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
