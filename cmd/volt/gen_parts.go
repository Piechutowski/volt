package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// genParts are the names -parts accepts, each one or more generated
// files (§V1.7): models (nao_models.go: structs, params structs, enums),
// queries (nao_queries.go, nao_dyn.go, nao_selects.go, nao_validate.go),
// router (volt_handlers.go, volt_router.go, volt_paths.go,
// volt_routes.go), client (volt_client.go) and sql (nao_schema.sql).
var genParts = []string{"models", "queries", "router", "client", "sql"}

// partsParse resolves the -parts list, or the defaults the older flags
// spell: every Go part, minus the query layers under --models-only,
// plus the DDL under --sql.
func partsParse(list string, modelsOnly, sql bool) (map[string]bool, error) {
	parts := map[string]bool{}
	if list == "" {
		parts["models"] = true
		parts["queries"], parts["router"], parts["client"] = !modelsOnly, true, true
	} else {
		for _, p := range strings.Split(list, ",") {
			p = strings.TrimSpace(p)
			if !slices.Contains(genParts, p) {
				return nil, fmt.Errorf("unknown part %q in -parts; the parts are %s", p, strings.Join(genParts, ", "))
			}
			parts[p] = true
		}
	}
	if sql {
		parts["sql"] = true
	}
	return parts, nil
}

// partOf names the part a generated file belongs to.
func partOf(name string) string {
	switch base := filepath.Base(name); {
	case base == "nao_models.go":
		return "models"
	case base == "nao_schema.sql":
		return "sql"
	case strings.HasPrefix(base, "nao_"):
		return "queries"
	case base == "volt_client.go":
		return "client"
	default:
		return "router"
	}
}

// goPackageName reads the package clause of the Go files in dir, the
// clause `volt gen -o` writes into that directory; "" when there are
// none. Test files are skipped, as their package may be the external one.
func goPackageName(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.PackageClauseOnly)
		if err != nil {
			continue
		}
		return f.Name.Name
	}
	return ""
}
