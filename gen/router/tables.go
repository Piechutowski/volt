package router

import (
	"strconv"

	"github.com/Piechutowski/volt/gen/align"
	"github.com/Piechutowski/volt/gen/layout"
	"github.com/Piechutowski/volt/lang"
)

func actionParams(a *lang.ActionInfo) []string {
	params := []string{"r volt.Request"}
	for _, sl := range a.ScopeLoads {
		params = append(params, sl.Name+" "+sl.GoType)
	}
	for _, p := range a.Params {
		params = append(params, p.GoName+" "+p.Type.GoType())
	}
	if a.Body != nil {
		params = append(params, "body "+a.Body.GoType())
	}
	for _, ld := range a.Loads {
		params = append(params, ld.Name+" "+ld.GoType())
	}
	return params
}

func (g *generator) handlersEmit() error {
	imports := []string{"net/http", "", voltImport, runImport}
	for _, q := range g.loadPackages() {
		imports = append(imports, dataImport(q))
	}
	imports = append(imports, g.scopeLoadImports(imports)...)
	g.header(imports...)

	for _, name := range g.controllerNames() {
		ci := g.pkg.Controllers[name]
		g.pf("type %sController interface {\n", name)
		for _, a := range ci.Actions {
			g.docWrite("\t", a.Doc)
			g.line(1, "", layout.List(a.Name+"(", texts(actionParams(a)...), ") volt.Response"), "")
		}
		g.pf("}\n\n")
	}

	g.pf("type Controllers struct {\n")
	var fields align.Block
	for _, name := range g.controllerNames() {
		fields.Row(name, name+"Controller")
	}
	fields.Row("DB", "run.DB")
	if g.hasEvents() {
		fields.Row("Events", "*volt.Broker")
	}
	fields.WriteTo(&g.out, "\t")
	g.pf("}\n\n")

	g.pf("func NewRouter(c Controllers) http.Handler {\n")
	g.pf("\tmux := http.NewServeMux()\n")
	g.pf("\tregister(mux, c)\n")
	g.pf("\treturn mux\n")
	g.pf("}\n\n")

	actions := 0
	for _, name := range g.controllerNames() {
		actions += len(g.pkg.Controllers[name].Actions)
	}
	if actions == 0 {
		g.pf("func ControllersDeterministic(_ Controllers, _ volt.Request) []string {\n\treturn nil\n}\n")
		return nil
	}
	g.pf("func ControllersDeterministic(c Controllers, r volt.Request) []string {\n")
	g.pf("\tvar voltActions []string\n")
	g.pf("\tvoltSame := func(voltAction string, voltCall func() volt.Response) {\n")
	g.pf("\t\tif !volt.Same(voltCall(), voltCall()) {\n")
	g.pf("\t\t\tvoltActions = append(voltActions, voltAction)\n\t\t}\n\t}\n")
	for _, name := range g.controllerNames() {
		for _, a := range g.pkg.Controllers[name].Actions {
			args := append([]string{"r"}, zeroScopeLoads(a.ScopeLoads)...)
			args = append(append(append(args, zeroArgs(a.Params)...), zeroBody(a.Body)...), zeroLoads(a.Loads)...)
			action := calls("c."+name+"."+a.Name, args...)
			called := layout.Func(nil, "volt.Response", layout.Prefixed("return ", action))
			same := call("voltSame", text(strconv.Quote(name+"."+a.Name)), called)
			g.line(1, "", same, "")
		}
	}
	g.pf("\treturn voltActions\n}\n")
	return nil
}

func (g *generator) pathsEmit() error {
	named := false
	for _, r := range g.pkg.Routes {
		if r.HelperName != "" {
			named = true
			break
		}
	}
	if named {
		g.header(voltImport)
	} else {
		g.header()
	}

	for _, r := range g.pkg.Routes {
		if r.HelperName == "" {
			continue
		}
		params := make([]string, 0, len(r.Params)+1)
		for _, p := range r.Params {
			params = append(params, p.GoName+" "+p.Type.GoType())
		}
		params = append(params, "voltOptions ...volt.URLOption")
		g.line(0, "", layout.List("func Path"+r.HelperName+"(", texts(params...), ") string {"), "")
		g.line(1, "return ", call("volt.URL", pathExpr(r), text("voltOptions...")), "")
		g.pf("}\n\n")
	}
	return nil
}

func (g *generator) routesEmit() error {
	g.header(voltImport)

	if len(g.pkg.Routes) == 0 {
		g.pf("var Table = []volt.Route{}\n")
		return nil
	}
	g.pf("var Table = []volt.Route{\n")
	for _, r := range g.pkg.Routes {
		keys, values := routeTableFields(r)
		g.line(1, "", layout.Fields("{", keys, values, "}"), ",")
	}
	g.pf("}\n")
	return nil
}

func routeTableFields(r *lang.RouteInfo) ([]string, []layout.Elem) {
	keys := []string{"Method", "Pattern", "Spelled"}
	values := texts(strconv.Quote(r.Method), strconv.Quote(r.Pattern), strconv.Quote(r.Spelled))
	if r.Query != nil {
		keys, values = append(keys, "Query"), append(values, text(strconv.Quote(r.Query.Ref())))
	} else {
		keys = append(keys, "Controller", "Action")
		values = append(values, texts(strconv.Quote(r.Controller), strconv.Quote(r.Action))...)
	}
	if r.HelperName != "" {
		keys, values = append(keys, "Helper"), append(values, text(strconv.Quote("Path"+r.HelperName)))
	}
	if len(r.Params) > 0 {
		params := make([]layout.Elem, len(r.Params))
		for i, p := range r.Params {
			pk, pv := []string{"Name", "Type"}, texts(strconv.Quote(p.Name), strconv.Quote(string(p.Type)))
			if p.Wild {
				pk, pv = append(pk, "Wild"), append(pv, text("true"))
			}
			params[i] = layout.Fields("{", pk, pv, "}")
		}
		keys, values = append(keys, "Params"), append(values, layout.List("[]volt.RouteParam{", params, "}"))
	}
	return keys, values
}
