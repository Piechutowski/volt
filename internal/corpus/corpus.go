// Package corpus writes synthetic Volt projects of any size that use
// every feature of the language — enums, partials, relations, preds,
// projected and group selects, typed and Go-reference checks,
// pipelines, error handlers, default resources, datasets, controller
// and query routes, the event stream — in a neutral domain, for the
// scaling, allocation and schedule tests and the benchmarks (PERF-2).
// One Spec, two layouts: data packages plus a routing package that
// imports them all, or one package holding everything (D76).
package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Spec sizes a project.
type Spec struct {
	Packages int  // data packages d01..dNN (1 in the one-package layout)
	Tables   int  // tables per data package
	Columns  int  // data columns per table, beyond the key and the partial
	Single   bool // one package holds schema and routes (D76)
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
	out := map[string]string{"go.mod": "module corpus\n\ngo 1.27\n"}
	if spec.Single {
		var b strings.Builder
		b.WriteString("package site\n\n")
		schemaWrite(&b, spec, "site")
		b.WriteString("\n")
		routesWrite(&b, spec, []string{"site"}, true)
		out["site/site.volt"] = b.String()
		out["site/checks.go"] = checksGo("site")
		out["site/mw.go"] = middlewareGo("site")
		return out
	}
	var pkgs []string
	for p := 1; p <= spec.Packages; p++ {
		name := fmt.Sprintf("d%02d", p)
		pkgs = append(pkgs, name)
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
	return out
}

// Table names the i-th table (1-based) of a data package.
func Table(i int) string { return fmt.Sprintf("tb%03ds", i) }

// colTypes cycle through the column types of Appendix A with and
// without NULL, so every model has every shape of field.
var colTypes = []string{
	"integer [not null]",
	"text [not null]",
	"real",
	"boolean [not null, default: false]",
	"status [not null, default: status.active]",
	"text",
	"timestamp",
	"bigint [not null]",
}

// schemaWrite renders the data declarations of one package: an enum,
// a partial every table injects, a pred every select reuses, the
// tables with a relation chain and checks, a group of every table, a
// projected select per table and the group select a dataset expands.
func schemaWrite(b *strings.Builder, spec Spec, pkg string) {
	b.WriteString("Enum status {\n\tactive\n\tretired [note: 'no longer written']\n}\n\n")
	b.WriteString("TablePartial stamped {\n\tcreated_at timestamp [not null, default: `CURRENT_TIMESTAMP`]\n}\n\n")
	b.WriteString("Pred fresh { c001 >= :since }\n\n")
	for i := 1; i <= spec.Tables; i++ {
		fmt.Fprintf(b, "Table %s [note: 'metrics table %d of %s'] {\n", Table(i), i, pkg)
		b.WriteString("\tid integer [pk, increment]\n\t~stamped\n")
		for c := 1; c <= spec.Columns; c++ {
			typ := colTypes[(c-1)%len(colTypes)]
			if c == 1 {
				typ = "integer [not null]" // the pred's column, every table
			}
			fmt.Fprintf(b, "\tc%03d %s\n", c, typ)
		}
		if i > 1 {
			fmt.Fprintf(b, "\tprev_id integer [ref: > %s.id]\n", Table(i-1))
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

// routesWrite renders the routing declarations: a pipeline of runtime
// and local middleware, a root scope with an error handler, a
// controller route, a wildcard route and the event stream, then per
// data package a named scope with a default resources per table and a
// dataset over the group select. In the one-package layout the
// references are bare (D76).
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
		fmt.Fprintf(b, "\t\tget /stats Stats.%s\n", strings.ToUpper(p[:1])+p[1:])
		for i := 1; i <= spec.Tables; i++ {
			fmt.Fprintf(b, "\t\tresources %s%s [default]\n", qual, Table(i))
		}
		fmt.Fprintf(b, "\t\tScope /browse [name: browse] {\n\t\t\tdataset %sbrowse\n\t\t}\n\t}\n", qual)
	}
	b.WriteString("}\n")
}

// checksGo is the Go file a data package's Go-reference check names.
func checksGo(pkg string) string {
	return "package " + pkg + `

import "errors"

// Positive is the Go-reference check every table names (§V12.5).
func Positive(v int32) error {
	if v < 0 {
		return errors.New("negative")
	}
	return nil
}
`
}

// middlewareGo is the Go file the pipeline and the error handler name.
func middlewareGo(pkg string) string {
	return "package " + pkg + `

import (
	"net/http"

	volt "github.com/Piechutowski/volt"
)

// BearerAuth is the plug the pipeline names (§V3.2).
func BearerAuth(next http.Handler) http.Handler { return next }

// Errors is the error_handler the root scope names (§V4.4).
func Errors(w http.ResponseWriter, r *volt.Request, err error) {
	volt.DefaultErrorHandler(w, r, err)
}
`
}
