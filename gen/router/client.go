package router

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Piechutowski/volt/gen/layout"
	"github.com/Piechutowski/volt/lang"
)

func (g *generator) clientEmit() error {
	var imports []string
	anyMethod, rawMethod := false, false
	for _, r := range g.pkg.Routes {
		if r.ClientName == "" {
			continue
		}
		anyMethod = true
		if r.Query == nil && !r.Events {
			rawMethod = true
		}
	}
	if anyMethod {
		imports = append(imports, "context")
	}
	if rawMethod {
		imports = append(imports, "net/http")
	}
	if g.clientTimeParams() {
		imports = append(imports, "time")
	}
	imports = append(imports, "", voltImport)
	if g.queryOptionalParams() {
		imports = append(imports, rtImport)
	}
	for _, q := range g.clientPackages() {
		if !g.opts.ClientBeside {
			imports = append(imports, clientImport(q))
		}
	}
	ctor := "New"
	if g.opts.ClientBeside {
		g.headerFor(g.pkgName(), imports...)
		ctor = "NewClient"
	} else {
		g.headerFor("client", imports...)
	}

	fixed := g.clientFixedNames()
	g.pf("type Client struct {\n\tvolt.Client\n}\n\n")
	g.pf("func %s(base string) *Client {\n", ctor)
	inner := layout.Fields("volt.Client{", []string{"Base", "Header"}, texts("base", "map[string][]string{}"), "}")
	g.line(1, "return ", layout.List("&Client{", []layout.Elem{inner}, "}"), "")
	g.pf("}\n\n")

	for _, r := range g.pkg.Routes {
		if r.ClientName != "" {
			g.clientMethodEmit(r, fixed)
		}
	}
	return nil
}

type clientFixed struct {
	client, context, changeset, body, query, value, err, url string
}

func (g *generator) clientFixedNames() clientFixed {
	var reserved []string
	if !g.opts.ClientBeside {
		for _, q := range g.clientPackages() {
			reserved = append(reserved, clientQualifier(q))
		}
	}
	n := localsAllocate(reserved, []string{"Client", "Context", "Changeset", "Body", "Query", "Value", "Err", "URL"})
	return clientFixed{
		client:    n[0],
		context:   n[1],
		changeset: n[2],
		body:      n[3],
		query:     n[4],
		value:     n[5],
		err:       n[6],
		url:       n[7],
	}
}

func (f clientFixed) all() []string {
	return []string{f.client, f.context, f.changeset, f.body, f.query, f.value, f.err, f.url}
}

type clientCall struct {
	params  []string
	query   clientQuery
	results string
	before  string
	fn      string
	in, out string
	result  string
}

func (g *generator) clientMethodEmit(r *lang.RouteInfo, fixed clientFixed) {
	names := g.clientNamesNew()
	pathNames := map[string]string{}
	pathParams := []string{fixed.context + " context.Context"}
	for _, p := range r.Params {
		pathNames[p.Name] = names.name(p.GoName)
		pathParams = append(pathParams, pathNames[p.Name]+" "+p.Type.GoType())
	}
	var c clientCall
	switch {
	case r.Events:
		query := clientQueryOf(fixed, pathNames, nil, nil)
		c = clientCall{params: pathParams, query: query, results: "<-chan volt.Event", before: "return ", fn: "Stream"}
	case r.Query == nil:
		c = g.clientRawCall(r, fixed, names, pathNames, pathParams)
	default:
		c = g.clientQueryCall(r, fixed, names, pathNames)
	}
	head := "func (" + fixed.client + " *Client) " + r.ClientName + "("
	g.line(0, "", layout.Signature(strings.TrimSuffix(head, "("), c.params, layout.Results(c.results), " {"), "")
	c.query.emit(g)
	method := r.Method
	if method == "" {
		method = "GET"
	}
	if c.result != "" {
		g.pf("\tvar %s %s\n", strings.TrimPrefix(c.out, "&"), c.result)
	}
	g.clientSend(r, fixed, c, method)
	if c.result != "" {
		g.pf("\treturn %s, %s\n", strings.TrimPrefix(c.out, "&"), fixed.err)
	}
	g.pf("}\n\n")
}

func (g *generator) clientSend(r *lang.RouteInfo, fixed clientFixed, c clientCall, method string) {
	args := []string{fixed.context}
	var tail []string
	if c.fn != "Stream" {
		args = append(args, strconv.Quote(method))
		tail = append(tail, c.in)
	}
	if c.out != "" {
		tail = append(tail, c.out)
	}
	url := call("volt.URL", c.query.urlArgs(r)...)
	sent := func(subject layout.Elem) layout.Elem {
		elems := append(texts(args...), subject)
		return call(fixed.client+"."+c.fn, append(elems, texts(tail...)...)...)
	}
	if inline := layout.Line(1, c.before, sent(url), ""); !strings.Contains(inline, "\n") {
		g.pf("\t%s\n", inline)
		return
	}
	g.line(1, fixed.url+" := ", url, "")
	g.line(1, c.before, sent(text(fixed.url)), "")
}

func (g *generator) clientRawCall(
	r *lang.RouteInfo, fixed clientFixed, names clientNames, pathNames map[string]string, pathParams []string,
) clientCall {
	keys := loadKeys(r)
	params := append([]string{}, pathParams...)
	keyNames := make([]string, len(keys))
	for i, p := range keys {
		keyNames[i] = names.name(p.Name)
		params = append(params, keyNames[i]+" "+p.Type())
	}
	in := "volt.NoBody"
	if b := r.Body; b != nil && b.Kind == lang.BodyChangeset {
		params = append(params, fixed.changeset+" "+g.clientType(b.Query, b.Query.Changeset.Type))
		in = fixed.changeset + ".Params()"
	} else if b != nil {
		params = append(params, fixed.body+" any")
		in = fixed.body
	}
	query := clientQueryOf(fixed, pathNames, keys, keyNames)
	results := "(*http.Response, error)"
	return clientCall{params: params, query: query, results: results, before: "return ", fn: "Raw", in: in}
}

func (g *generator) clientQueryCall(
	r *lang.RouteInfo, fixed clientFixed, names clientNames, pathNames map[string]string,
) clientCall {
	q := r.Query
	params := []string{fixed.context + " context.Context"}
	paramNames := make([]string, len(q.Params))
	in := "volt.NoBody"
	for i, p := range q.Params {
		goType := p.Type()
		switch p.Source {
		case lang.FromBody:
			goType = g.clientType(q, strings.TrimPrefix(goType, q.Qualifier+"."))
			paramNames[i], in = fixed.changeset, fixed.changeset+".Params()"
		case lang.FromPath:
			paramNames[i] = pathNames[p.Name]
		default:
			paramNames[i] = names.name(p.Name)
		}
		params = append(params, paramNames[i]+" "+goType)
	}
	query := clientQueryOf(fixed, pathNames, q.Params, paramNames)
	if q.Result == "" {
		return clientCall{
			params: params, query: query, results: "error", before: "return ", fn: "Do", in: in, out: "volt.NoBody",
		}
	}
	result, base := g.clientType(q, q.Result), q.Result
	if q.Many {
		result, base = "[]"+result, localPlural(base)
	}
	out := localsAllocate(fixed.all(), []string{base})[0]
	return clientCall{
		params: params, query: query, results: "(" + result + ", error)",
		before: fixed.err + " := ", fn: "Do", in: in, out: "&" + out, result: result,
	}
}

// clientQuery collects what a client method sends in the query string
// (spec/generated-client): the options of required values, and the
// statements that add the optional ones and the lists.
type clientQuery struct {
	opts  []layout.Elem
	steps []string
	path  map[string]string
	fixed clientFixed
}

func clientQueryOf(fixed clientFixed, path map[string]string, params []lang.QueryParam, names []string) clientQuery {
	var opts []layout.Elem
	var steps []string
	query, value := fixed.query, fixed.value
	for i, p := range params {
		name := names[i]
		format := func(v string) layout.Elem {
			return call("volt.Query", text(strconv.Quote(p.Name)), call("volt.FormatQuery", text(v)))
		}
		appended := func(v string) string {
			return layout.Line(2, query+" = ", call("append", text(query), format(v)), "")
		}
		switch {
		case p.Source == lang.FromQuery && p.Optional:
			steps = append(steps, fmt.Sprintf("\tif %s.Valid {\n\t\t%s\n\t}\n", name, appended(name+".V")))
		case p.Source == lang.FromQuery:
			opts = append(opts, format(name))
		case p.Source == lang.FromList:
			steps = append(steps, fmt.Sprintf("\tfor _, %s := range %s {\n\t\t%s\n\t}\n", value, name, appended(value)))
		}
	}
	return clientQuery{opts: opts, steps: steps, path: path, fixed: fixed}
}

func (q clientQuery) emit(g *generator) {
	if len(q.opts) == 0 && len(q.steps) == 0 {
		return
	}
	g.line(1, q.fixed.query+" := ", layout.List("[]volt.URLOption{", q.opts, "}"), "")
	g.pf("%s", strings.Join(q.steps, ""))
}

func (q clientQuery) urlArgs(r *lang.RouteInfo) []layout.Elem {
	if len(q.opts) == 0 && len(q.steps) == 0 {
		return []layout.Elem{pathExprNamed(r, q.path)}
	}
	return []layout.Elem{pathExprNamed(r, q.path), text(q.fixed.query + "...")}
}
