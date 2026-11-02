package lang

import (
	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/nao/naming"
)

func bareTypesDeclared(f *ast.File) bool {
	for _, d := range f.Decls {
		if td, ok := d.(*ast.TypeDecl); ok && td.Name != nil && td.Name.Qualifier() == "" {
			return true
		}
	}
	return false
}

func (c *checker) typesGoCheck(pkg *Package, info *check.Info, sc *goScan) {
	names := pkg.plan.Names()
	for _, td := range info.Types {
		if td.Path != "" {
			continue
		}
		if _, declared := sc.types[td.Key]; !declared {
			storage := td.Storage.Storage
			if check.KindedStorage[storage] {
				c.errorf(td.Decl.Name.Pos(), "types", "Type %s is declared in no Go file of package %s; write type %s %s beside the schema%s (§types)",
					td.Key, pkg.Name, td.Key, storage, sc.brokenTypesNote())
			} else {
				c.errorf(td.Decl.Name.Pos(), "types", "Type %s is declared in no Go file of package %s; write type %s %s beside the schema, with a Scan method on its pointer and a Value method%s (§types)",
					td.Key, pkg.Name, td.Key, storage, sc.brokenTypesNote())
			}
		}
		if desc, taken := names.Generated(td.Key); taken {
			c.errorf(td.Decl.Name.Pos(), "types", "Type %s takes the Go name of %s; a declared type is named apart from the names generated code declares, e.g. %s (§types)",
				td.Key, desc, typeNameFree(names, td.Key))
		} else if desc, taken := migrationNames[td.Key]; taken && len(info.Migrations) > 0 {
			c.errorf(td.Decl.Name.Pos(), "types", "Type %s takes the Go name of %s; a declared type is named apart from the names generated code declares (§types)", td.Key, desc)
		}
	}
}

var migrationNames = map[string]string{
	"Migrations": "the generated migration list",
	"Migrate":    "the generated migration runner",
}

func (c *checker) typesRouterCheck(pkg *Package) {
	if pkg.schema == nil || !pkg.HasRouting() {
		return
	}
	router := map[string]string{
		"Controllers": "the generated Controllers manifest",
		"NewRouter":   "the generated router constructor",
		"Table":       "the generated route table",
		"Client":      "the generated client type, which gen -o writes beside the models",
		"NewClient":   "the generated client constructor, which gen -o writes beside the models",
	}
	for name := range pkg.Controllers {
		router[name+"Controller"] = "the generated controller interface of " + name
	}
	for _, r := range pkg.Routes {
		if r.HelperName != "" {
			router["Path"+r.HelperName] = "the generated reverse-URL helper of " + methodOrAny(r.Method) + " " + r.Spelled
		}
	}
	for _, td := range pkg.schema.Types {
		if td.Path != "" {
			continue
		}
		if desc, taken := router[td.Key]; taken {
			c.errorf(td.Decl.Name.Pos(), "types", "Type %s takes the Go name of %s; a declared type is named apart from the names generated code declares (§types)", td.Key, desc)
		}
	}
}

func typeNameFree(names *naming.Names, name string) string {
	for _, suffix := range []string{"Kind", "Value", "Type"} {
		if _, taken := names.Lookup(name + suffix); !taken {
			return name + suffix
		}
	}
	return name + "Kind"
}
