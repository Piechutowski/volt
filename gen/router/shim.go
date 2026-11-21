package router

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Piechutowski/volt/gen/layout"
	"github.com/Piechutowski/volt/lang"
)

func call(fn string, args ...layout.Elem) layout.Elem { return layout.Call(fn, args) }

func texts(ss ...string) []layout.Elem { return layout.Texts(ss) }

var text = layout.Text

func (g *generator) line(indent int, before string, e layout.Elem, after string) {
	layout.WriteLine(&g.out, indent, before, e, after+"\n")
}

func (g *generator) failOnErr(indent int, head string) {
	tabs := layout.Tabs(indent)
	g.pf("%s%s {\n%s\treturn volt.Fail(err)\n%s}\n", tabs, head, tabs, tabs)
}

func (g *generator) continuation(indent int, fn string, effect layout.Elem, name, param, typ string) (int, string) {
	tabs := layout.Tabs(indent)
	next := "func(" + strings.TrimSpace(param+" "+typ) + ") volt.Response {"
	head := func(subject string) string { return "return " + fn + "(" + subject + ", " + next }
	subject := layout.Render(effect, layout.At{OneLine: true})
	if layout.Fits(tabs+head(subject)) && !strings.Contains(subject, "\n") {
		g.pf("%s%s\n", tabs, head(subject))
		return indent + 1, tabs + "})\n"
	}
	g.line(indent, name+" := ", effect, "")
	if layout.Fits(tabs + head(name)) {
		g.pf("%s%s\n", tabs, head(name))
		return indent + 1, tabs + "})\n"
	}
	if sig := strings.TrimSpace(param + " " + typ); !layout.Fits(tabs+"\t"+next) && sig != "" {
		next = "func(\n" + tabs + "\t\t" + sig + ",\n" + tabs + "\t) volt.Response {"
	}
	g.pf("%sreturn %s(\n%s\t%s,\n%s\t%s\n", tabs, fn, tabs, name, tabs, next)
	return indent + 2, tabs + "\t},\n" + tabs + ")\n"
}

func (g *generator) routeEmit(index int, r *lang.RouteInfo) {
	plugs := g.routePlugs(r)
	controllers, perform := "c", "perform"
	if r.Query != nil && !r.Events {
		controllers = "_"
	}
	if r.Events {
		perform = "_"
	}
	g.pf("\nfunc voltRoute%d(%s Controllers, %s volt.Performer) http.Handler {\n", index, controllers, perform)
	if r.Events {
		g.handlerReturn(call("volt.EventsHandler", text("c.Events")), plugs)
		return
	}
	g.pf("\tshim := func(r volt.Request) volt.Response {\n")
	route{g: g, r: r, names: routeLocalsOf(r)}.shimEmit()
	g.pf("\t}\n")
	g.handlerReturn(routeHandlerCall(r), plugs)
}

func (g *generator) handlerReturn(handler layout.Elem, plugs []string) {
	if len(plugs) == 0 {
		g.line(1, "return ", handler, "")
		g.pf("}\n")
		return
	}
	g.line(1, "voltHandler := ", handler, "")
	g.line(1, "return ", calls("volt.Chain", append([]string{"voltHandler"}, plugs...)...), "")
	g.pf("}\n")
}

func (g *generator) routePlugs(r *lang.RouteInfo) []string {
	var plugs []string
	for _, name := range r.Pipes {
		for _, plug := range g.pkg.Pipelines[name].Plugs {
			plugs = append(plugs, plug.Ref.String())
		}
	}
	return plugs
}

func routeHandlerCall(r *lang.RouteInfo) layout.Elem {
	eh := "volt.DefaultErrorHandler"
	if r.ErrorHandler != "" {
		eh = "errHandler" + r.ErrorHandler
	}
	args := texts(strconv.Quote(fullPattern(r)), "shim", eh, "perform")
	if len(r.Params) > 0 {
		names := make([]string, len(r.Params))
		for i, p := range r.Params {
			names[i] = strconv.Quote(p.Name)
		}
		args = append(args, calls("volt.Params", names...))
	}
	if r.Timeout > 0 {
		args = append(args, text(fmt.Sprintf("volt.Timeout(%d*time.Second)", r.Timeout)))
	}
	return call("volt.Handler", args...)
}

type route struct {
	g     *generator
	r     *lang.RouteInfo
	names routeLocals
}

func (e route) shimEmit() {
	g, r := e.g, e.r
	parsed := map[string]string{}
	pathArgs := make([]string, 0, len(r.Params))
	if r.Query != nil {
		g.failOnErr(2, "if err := volt.Accepts(r); err != nil")
	}
	for _, p := range r.Params {
		parsed[p.Name] = e.pathParse(p)
		pathArgs = append(pathArgs, parsed[p.Name])
	}
	scoped := make([]string, len(r.ScopeLoads))
	for i := range r.ScopeLoads {
		scoped[i] = "_"
		if r.Query == nil {
			scoped[i] = e.names[fmt.Sprintf("scope:%d", i)]
		}
	}
	loaded := make([]string, len(r.Loads))
	for i := range r.Loads {
		loaded[i] = e.names[fmt.Sprintf("load:%d", i)]
	}
	g.pathLimitsEmit(r, parsed)
	indent := 2
	var closers []string
	for i, sl := range r.ScopeLoads {
		var closer string
		hoist := e.names[fmt.Sprintf("hoist:scope:%d", i)]
		indent, closer = g.continuation(indent, "volt.Continue", call(sl.Func, text("r")), hoist, scoped[i], sl.GoType)
		closers = append(closers, closer)
	}
	if r.Query != nil {
		e.queryShimBody(indent, parsed)
	} else {
		args := append([]string{"r"}, scoped...)
		args = append(args, pathArgs...)
		body := ""
		if r.Body != nil {
			body = e.names["Body"]
			args = append(args, body)
		}
		args = append(args, loaded...)
		handler := calls("c."+r.Controller+"."+r.Action, args...)
		inner, closer := e.bodyEmit(indent, parsed, body)
		e.loadsEmit(inner, parsed, loaded, handler)
		g.pf("%s", closer)
	}
	for i := len(closers) - 1; i >= 0; i-- {
		g.pf("%s", closers[i])
	}
}

func (e route) pathParse(p lang.Param) string {
	g := e.g
	if p.Type == lang.TString {
		return fmt.Sprintf("r.PathValue(%q)", p.Name)
	}
	v := e.names["path:"+p.Name]
	g.line(2, v+", ok := ", call("volt.ParseInt", call("r.PathValue", text(strconv.Quote(p.Name)))), "")
	g.pf("\t\tif !ok {\n\t\t\treturn volt.Fail(volt.ErrNotFound)\n\t\t}\n")
	return v
}

func qualified(q *lang.QueryRef, name string) string {
	if q.Local {
		return name
	}
	return q.Qualifier + "." + name
}

func (e route) queryShimBody(indent int, parsed map[string]string) {
	g, q := e.g, e.r.Query
	args := make([]string, 0, len(q.Params))
	var closer string
	for _, p := range q.Params {
		switch p.Source {
		case lang.FromPath:
			args = append(args, parsed[p.Name])
		case lang.FromQuery:
			v := e.names["query:"+p.Name]
			g.queryBindEmit(indent, p, v)
			args = append(args, v)
		case lang.FromList:
			v := e.names["query:"+p.Name]
			g.listBindEmit(indent, p, v)
			args = append(args, v)
		case lang.FromBody:
			cb := q.Changeset
			params, changeset := e.names["Params"], e.names["Changeset"]
			g.decodeEmit(indent, params, qualified(q, cb.Params))
			start := qualified(q, cb.New) + "()"
			if cb.Get != "" {
				row := e.names["Row"]
				get := calls(qualified(q, cb.Get), args...)
				hoist := e.names["hoist:get"]
				indent, closer = g.continuation(indent, "volt.Then", get, hoist, row, qualified(q, q.Result))
				args, start = nil, row+".Changeset()"
			}
			cast := append([]string{params}, g.allowed(q, cb.Allow)...)
			g.line(indent, changeset+" := ", calls(start+".Cast", cast...), "")
			args = append(args, changeset)
		}
	}
	effect := calls(qualified(q, q.Method), args...)
	status := statusName(q.Status)
	param, value, typ := "", "volt.NoBody", "int64"
	if q.Result != "" {
		value = e.names["result"]
		param, typ = value+" ", g.queryResult(q)
	}
	body, done := g.continuation(indent, "volt.Then", effect, e.names["hoist:query"], strings.TrimSpace(param), typ)
	g.line(body, "return ", calls("volt.RenderStatus", status, value), "")
	g.pf("%s%s", done, closer)
}

func (g *generator) allowed(q *lang.QueryRef, allow []string) []string {
	out := make([]string, len(allow))
	for i, h := range allow {
		out[i] = qualified(q, h)
	}
	return out
}

func (g *generator) decodeEmit(indent int, v, typ string) {
	g.pf("%svar %s %s\n", layout.Tabs(indent), v, typ)
	g.failOnErr(indent, "if err := volt.Decode(r, &"+v+"); err != nil")
}

func queryBinder(p lang.QueryParam) string {
	if p.Optional {
		return "QueryParamOptional"
	}
	return "QueryParam"
}

func (g *generator) queryBindEmit(indent int, p lang.QueryParam, v string) {
	g.line(indent, v+", err := ", calls("volt."+queryBinder(p)+"["+p.GoType+"]", "r", strconv.Quote(p.Name)), "")
	g.failOnErr(indent, "if err != nil")
	g.limitCheckEmit(indent, p, v)
}

func (g *generator) listBindEmit(indent int, p lang.QueryParam, v string) {
	binder := "volt.QueryParams[" + strings.TrimPrefix(p.GoType, "[]") + "]"
	g.line(indent, v+", err := ", calls(binder, "r", strconv.Quote(p.Name)), "")
	g.failOnErr(indent, "if err != nil")
}

func (g *generator) limitCheckEmit(indent int, p lang.QueryParam, v string) {
	if !p.Limit {
		return
	}
	check := calls("volt.LimitCheck", strconv.Quote(p.Name), v)
	g.pf("%s%s\n", layout.Tabs(indent), layout.Line(indent, "if err := ", check, "; err != nil {"))
	g.pf("%s\treturn volt.Fail(err)\n%s}\n", layout.Tabs(indent), layout.Tabs(indent))
}

func (g *generator) pathLimitsEmit(r *lang.RouteInfo, parsed map[string]string) {
	limits := map[string]bool{}
	note := func(q *lang.QueryRef) {
		for _, p := range q.Params {
			if p.Source == lang.FromPath && p.Limit {
				limits[p.Name] = true
			}
		}
	}
	if r.Query != nil {
		note(r.Query)
	}
	for _, ld := range r.Loads {
		note(ld.Query)
	}
	for _, p := range r.Params {
		if limits[p.Name] {
			g.limitCheckEmit(2, lang.QueryParam{Name: p.Name, Limit: true}, parsed[p.Name])
		}
	}
}

func (e route) loadsEmit(indent int, parsed map[string]string, names []string, handler layout.Elem) {
	g, r := e.g, e.r
	argsOf := make([][]string, len(r.Loads))
	for i, ld := range r.Loads {
		argsOf[i] = make([]string, len(ld.Query.Params))
		for j, p := range ld.Query.Params {
			switch p.Source {
			case lang.FromPath:
				argsOf[i][j] = parsed[p.Name]
			case lang.FromLiteral:
				argsOf[i][j] = p.Literal
			case lang.FromList:
				v := e.names[fmt.Sprintf("arg:%d:%d", i, j)]
				g.listBindEmit(indent, p, v)
				argsOf[i][j] = v
			default:
				v := e.names[fmt.Sprintf("arg:%d:%d", i, j)]
				g.queryBindEmit(indent, p, v)
				argsOf[i][j] = v
			}
		}
	}
	var closers []string
	for i, ld := range r.Loads {
		q := ld.Query
		effect := calls(qualified(q, q.Method), argsOf[i]...)
		var closer string
		hoist := e.names[fmt.Sprintf("hoist:load:%d", i)]
		indent, closer = g.continuation(indent, "volt.Then", effect, hoist, names[i], ld.GoType())
		closers = append(closers, closer)
	}
	g.line(indent, "return ", handler, "")
	for i := len(closers) - 1; i >= 0; i-- {
		g.pf("%s", closers[i])
	}
}

func (e route) bodyEmit(indent int, parsed map[string]string, body string) (int, string) {
	g, b := e.g, e.r.Body
	if b == nil {
		return indent, ""
	}
	if b.Kind == lang.BodyStruct {
		g.decodeEmit(indent, body, b.Struct)
		return indent, ""
	}
	q := b.Query
	cb := q.Changeset
	params := e.names["Params"]
	g.decodeEmit(indent, params, qualified(q, cb.Params))
	start, closer := qualified(q, cb.New)+"()", ""
	if cb.Get != "" {
		args := make([]string, len(q.Params))
		for i, p := range q.Params {
			if p.Source == lang.FromLiteral {
				args[i] = p.Literal
			} else {
				args[i] = parsed[p.Name]
			}
		}
		row := e.names["Row"]
		get := calls(qualified(q, cb.Get), args...)
		indent, closer = g.continuation(indent, "volt.Then", get, e.names["hoist:get"], row, qualified(q, q.Result))
		start = row + ".Changeset()"
	}
	cast := append([]string{params}, g.allowed(q, cb.Allow)...)
	g.line(indent, body+" := ", calls(start+".Cast", cast...), ".Validate()")
	return indent, closer
}
func calls(fn string, args ...string) layout.Elem { return layout.CallStrings(fn, args) }
