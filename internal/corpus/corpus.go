// Package corpus writes synthetic Volt projects of any size that use
// every feature of the language in a neutral domain. One
// Spec, two layouts: data packages plus a routing package that imports
// them all, or one package main at the module root holding everything
// in one schema.volt.
package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Spec sizes a project.
type Spec struct {
	Packages int    // data packages d01..dNN (ignored in the one-package layout)
	Tables   int    // tables per data package
	Columns  int    // data columns per table, beyond the key and the partial
	Single   bool   // one package main at the root holds schema and routes
	Volt     string // Volt checkout the module's runtime is replaced with; "" writes a module that checks and generates but does not build
}

// Write materializes the project under root: go.mod, every .volt file
// and the Go files the references name.
func Write(root string, spec Spec) error {
	for rel, body := range Files(spec) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Files renders the project as relative path -> content.
func Files(spec Spec) map[string]string {
	out := map[string]string{"go.mod": goMod(spec.Volt)}
	if spec.Single {
		var b strings.Builder
		b.WriteString("package main\n\n")
		schemaWrite(&b, spec, "site")
		b.WriteString("\n")
		routesWrite(&b, spec, []string{"site"}, true)
		out["schema.volt"] = b.String()
		out["checks.go"] = checksGo("main")
		out["mw.go"] = middlewareGo("main")
		out["main.go"] = controllersGo("main", []string{"Site"}, true)
		return out
	}
	var pkgs, stats []string
	for p := 1; p <= spec.Packages; p++ {
		name := fmt.Sprintf("d%02d", p)
		pkgs = append(pkgs, name)
		stats = append(stats, actionOf(name))
		var b strings.Builder
		fmt.Fprintf(&b, "package %s\n\n", name)
		schemaWrite(&b, spec, name)
		out[name+"/schema.volt"] = b.String()
		out[name+"/checks.go"] = checksGo(name)
	}
	var b strings.Builder
	b.WriteString("package app\n\n")
	routesWrite(&b, spec, pkgs, false)
	out["app/routes.volt"] = b.String()
	out["app/mw.go"] = middlewareGo("app")
	out["app/controllers.go"] = controllersGo("app", stats, false)
	return out
}

func goMod(volt string) string {
	s := "module corpus\n\ngo 1.27\n"
	if volt != "" {
		s += "\nrequire github.com/Piechutowski/volt v0.0.0\n\nreplace github.com/Piechutowski/volt => " + volt + "\n"
	}
	return s
}

func actionOf(scope string) string { return strings.ToUpper(scope[:1]) + scope[1:] }

// Table names the i-th table (1-based) of a data package.
func Table(i int) string { return fmt.Sprintf("tb%03ds", i) }

var colTypes = []string{
	"int [not null]",
	"string [not null]",
	"float64",
	"bool [not null, default: false]",
	"status [not null, default: status.active]",
	"string",
	"time.Time",
	"int [not null]",
}

func schemaWrite(b *strings.Builder, spec Spec, pkg string) {
	b.WriteString("Enum status {\n\tactive\n\tretired [note: 'no longer written']\n}\n\n")
	b.WriteString("TablePartial stamped {\n\tcreated_at time.Time [not null, default: `CURRENT_TIMESTAMP`]\n}\n\n")
	b.WriteString("Pred fresh { c001 >= :since }\n\n")
	for i := 1; i <= spec.Tables; i++ {
		fmt.Fprintf(b, "Table %s [note: 'metrics table %d of %s'] {\n", Table(i), i, pkg)
		b.WriteString("\tid int [pk, increment]\n\t~stamped\n")
		for c := 1; c <= spec.Columns; c++ {
			typ := colTypes[(c-1)%len(colTypes)]
			if c == 1 {
				typ = "int [not null]"
			}
			fmt.Fprintf(b, "\tc%03d %s\n", c, typ)
		}
		if i > 1 {
			fmt.Fprintf(b, "\tprev_id int [ref: > %s.id]\n", Table(i-1))
		}
		b.WriteString("\n\tchecks {\n\t\tc001 >= 0 [name: 'c001_nonneg']\n\t\tPositive(c001)\n\t}\n}\n\n")
	}
	b.WriteString("Group series {\n")
	for i := 1; i <= spec.Tables; i++ {
		fmt.Fprintf(b, "\t%s\n", Table(i))
	}
	b.WriteString("}\n\n")
	for i := 1; i <= spec.Tables; i++ {
		fmt.Fprintf(b, "Select brief%03d (id, c001) for %s where fresh [order: (id desc)]\n", i, Table(i))
	}
	b.WriteString("\nSelect browse for series where fresh [order: (id asc)]\n")
}

func routesWrite(b *strings.Builder, spec Spec, pkgs []string, single bool) {
	if !single {
		b.WriteString("import (\n")
		for _, p := range pkgs {
			fmt.Fprintf(b, "\t%s\n", p)
		}
		b.WriteString(")\n\n")
	}
	b.WriteString("Pipeline api {\n\tuse volt.RequestID\n\tuse BearerAuth\n}\n\n")
	b.WriteString("Scope / [pipe: api, error_handler: Errors] {\n")
	b.WriteString("\tget /              Home.Index [name: root]\n")
	b.WriteString("\tget /files/:path... Files.Serve\n")
	b.WriteString("\tget /events        volt.Events\n\n")
	for _, p := range pkgs {
		qual := p + "."
		if single {
			qual = ""
		}
		fmt.Fprintf(b, "\tScope /%s [name: %s] {\n", p, p)
		fmt.Fprintf(b, "\t\tget /stats Stats.%s\n", actionOf(p))
		for i := 1; i <= spec.Tables; i++ {
			fmt.Fprintf(b, "\t\tresources %s%s [default]\n", qual, Table(i))
		}
		fmt.Fprintf(b, "\t\tScope /browse [name: browse] {\n\t\t\tdataset %sbrowse\n\t\t}\n\t}\n", qual)
	}
	b.WriteString("}\n")
}

func checksGo(pkg string) string {
	return "package " + pkg + `

import "errors"

// Positive is the Go-reference check every table names (§validation-checks).
func Positive(v int) error {
	if v < 0 {
		return errors.New("negative")
	}
	return nil
}
`
}

func middlewareGo(pkg string) string {
	return "package " + pkg + `

import (
	"net/http"

	volt "github.com/Piechutowski/volt"
)

// BearerAuth is the plug the pipeline names (§pipelines).
func BearerAuth(next http.Handler) http.Handler { return next }

// Errors is the error_handler the root scope names (§scopes).
func Errors(w http.ResponseWriter, r *volt.Request, err error) {
	volt.DefaultErrorHandler(w, r, err)
}
`
}

func controllersGo(pkg string, stats []string, main bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\nimport (\n\t\"net/http\"\n\n\tvolt \"github.com/Piechutowski/volt\"\n)\n\n", pkg)
	b.WriteString("// stub answers every controller action the routes name.\ntype stub struct{}\n\n")
	b.WriteString("func (stub) Index(w http.ResponseWriter, r *volt.Request) error { return nil }\n\n")
	b.WriteString("func (stub) Serve(w http.ResponseWriter, r *volt.Request, path string) error { return nil }\n")
	for _, a := range stats {
		fmt.Fprintf(&b, "\nfunc (stub) %s(w http.ResponseWriter, r *volt.Request) error { return nil }\n", a)
	}
	if main {
		b.WriteString("\nfunc main() {\n\thttp.ListenAndServe(\":0\", NewRouter(Controllers{Home: stub{}, Files: stub{}, Stats: stub{}, Queries: New(nil)}))\n}\n")
	} else {
		b.WriteString("\nvar _ http.Handler = NewRouter(Controllers{Home: stub{}, Files: stub{}, Stats: stub{}})\n")
	}
	return b.String()
}
