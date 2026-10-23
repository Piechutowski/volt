package golang

import (
	"bytes"
	"flag"
	goast "go/ast"
	"go/format"
	goparser "go/parser"
	gotoken "go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

var update = flag.Bool("update", false, "rewrite golden files")

func parseChecked(t *testing.T, path string) (*ast.File, *check.Info) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, diags := parser.ParseFile(filepath.Base(path), string(src))
	info, semDiags := check.FileGo(f, nil, sidecarTypes(t, path))
	diags = append(diags, semDiags...)
	if diag.HasErrors(diags) {
		t.Fatalf("test input must check clean: %v", diags)
	}
	return f, info
}

// sidecarTypes reads the Go declarations a corpus schema's bare Types
// name, from the hand-written file beside it (x.volt, x.go), as the
// project pass reads a package's Go files (spec §types); nil when the
// schema has none.
func sidecarTypes(t *testing.T, path string) check.GoTypes {
	t.Helper()
	src, err := os.ReadFile(sidecarOf(path))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	gf, err := goparser.ParseFile(gotoken.NewFileSet(), sidecarOf(path), src, goparser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	out := check.GoTypes{}
	for _, d := range gf.Decls {
		gd, ok := d.(*goast.GenDecl)
		if !ok || gd.Tok != gotoken.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts := spec.(*goast.TypeSpec)
			out[ts.Name.Name] = types.ExprString(ts.Type)
		}
	}
	return out
}

func sidecarOf(schema string) string { return strings.TrimSuffix(schema, ".volt") + ".go" }

func generate(t *testing.T, path string) []byte {
	t.Helper()
	f, info := parseChecked(t, path)
	out, err := Generate(f, info, Options{Package: "models", Source: filepath.Base(path)})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return out
}

func generateQueries(t *testing.T, path string) []byte {
	t.Helper()
	f, info := parseChecked(t, path)
	out, err := GenerateQueries(f, info, Options{Package: "models", Source: filepath.Base(path)})
	if err != nil {
		t.Fatalf("GenerateQueries: %v", err)
	}
	return out
}

func generateDyn(t *testing.T, path string) []byte {
	t.Helper()
	f, info := parseChecked(t, path)
	out, err := GenerateDyn(f, info, Options{Package: "models", Source: filepath.Base(path)})
	if err != nil {
		t.Fatalf("GenerateDyn: %v", err)
	}
	return out
}

func generateValidators(t *testing.T, path string) []byte {
	t.Helper()
	f, info := parseChecked(t, path)
	out, err := GenerateValidators(f, info, validatorFns(t, f, info), Options{Package: "models", Source: filepath.Base(path)})
	if err != nil {
		t.Fatalf("GenerateValidators: %v", err)
	}
	return out
}

func validatorFns(t *testing.T, f *ast.File, info *check.Info) []ValidatorFn {
	t.Helper()
	var fns []ValidatorFn
	for _, ti := range info.Tables {
		if len(ti.Validators) == 0 {
			continue
		}
		_, fields, err := ModelFields(f, info, ti.Key)
		if err != nil {
			t.Fatalf("ModelFields(%s): %v", ti.Key, err)
		}
		byCol := map[string]string{}
		for _, fs := range fields {
			byCol[fs.Col] = fs.Name
		}
		fn := ValidatorFn{TableKey: ti.Key}
		for _, v := range ti.Validators {
			var args, src, cols []string
			for _, a := range v.Args {
				field, ok := byCol[a.Name()]
				if !ok {
					t.Fatalf("validator %s of %s names no column %q", v.Ref.String(), ti.Key, a.Name())
				}
				args = append(args, "v."+field)
				src = append(src, a.Name())
				if !slices.Contains(cols, a.Name()) {
					cols = append(cols, a.Name())
				}
			}
			name := v.Ref.Base()
			fn.Validators = append(fn.Validators, ValidatorSpec{
				Name: name,
				Src:  name + "(" + strings.Join(src, ", ") + ")",
				Call: name + "(" + strings.Join(args, ", ") + ")",
				Cols: cols,
			})
		}
		fns = append(fns, fn)
	}
	return fns
}

func compareGolden(t *testing.T, got []byte, golden string) {
	t.Helper()
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("missing golden file (run 'go test ./gen/... -update'): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from %s:\n--- got ---\n%s", golden, got)
	}
}

func TestGolden(t *testing.T) {
	for _, schema := range corpusSchemas(t) {
		schema := schema
		name := schemaName(schema)
		t.Run(name, func(t *testing.T) {
			compareGolden(t, generate(t, schema), filepath.Join("testdata", name+".go.golden"))
			compareGolden(t, generateQueries(t, schema), filepath.Join("testdata", name+"_queries.go.golden"))
			compareGolden(t, generateDyn(t, schema), filepath.Join("testdata", name+"_dyn.go.golden"))
			compareGolden(t, generateValidators(t, schema), filepath.Join("testdata", name+"_validate.go.golden"))
		})
	}
}

func corpusSchemas(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "testdata", "*.volt"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("no shared corpus schemas in gen/testdata")
	}
	return files
}

func schemaName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".volt")
}

func TestGoldenGofmtStable(t *testing.T) {
	for _, golden := range goldenFiles(t) {
		src, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		if !gofmtStable(t, src) {
			t.Errorf("%s is not gofmt-stable", golden)
		}
	}
}

func gofmtStable(t *testing.T, src []byte) bool {
	t.Helper()
	formatted, err := format.Source(src)
	if err != nil {
		t.Errorf("output does not parse: %v", err)
		return false
	}
	return bytes.Equal(src, formatted)
}

func TestGoldenCompiles(t *testing.T) {
	dir := t.TempDir()
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	gomod := "module goldencheck\n\ngo 1.27\n\n" +
		"require github.com/Piechutowski/volt v0.0.0\n\n" +
		"replace github.com/Piechutowski/volt => " + repoRoot + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, golden := range goldenFiles(t) {
		src, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		base := filepath.Base(strings.TrimSuffix(golden, ".go.golden"))
		outName := "nao_models.go"
		switch {
		case strings.HasSuffix(base, "_queries"):
			base, outName = strings.TrimSuffix(base, "_queries"), "nao_queries.go"
		case strings.HasSuffix(base, "_dyn"):
			base, outName = strings.TrimSuffix(base, "_dyn"), "nao_dyn.go"
		case strings.HasSuffix(base, "_validate"):
			base, outName = strings.TrimSuffix(base, "_validate"), "nao_validate.go"
		case strings.HasSuffix(base, "_selects"):
			base, outName = strings.TrimSuffix(base, "_selects"), "nao_selects.go"
		case strings.HasSuffix(base, "_migrations"):
			base, outName = strings.TrimSuffix(base, "_migrations"), "volt_migrations.go"
		}
		pkgDir := filepath.Join(dir, "p", base)
		if err := os.MkdirAll(pkgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkgDir, outName), src, 0o644); err != nil {
			t.Fatal(err)
		}
		if decls, err := os.ReadFile(sidecarOf(filepath.Join("..", "testdata", base+".volt"))); err == nil {
			if err := os.WriteFile(filepath.Join(pkgDir, "types.go"), decls, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("golden files do not compile:\n%s", out)
	}
}

// TestTypeImportAliasCompiles proves generated code imports a Type's
// package as `import pkg "path"` (spec §types) when the package's
// declared name is not the path's last element: a directory cash that
// declares package money, and a versioned path ending in v2.
func TestTypeImportAliasCompiles(t *testing.T) {
	dir := t.TempDir()
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	scanner := func(pkg, typ string) string {
		return "package " + pkg + "\n\nimport \"database/sql/driver\"\n\n" +
			"type " + typ + " int\n\n" +
			"func (a *" + typ + ") Scan(src any) error { *a = " + typ + "(src.(int64)); return nil }\n\n" +
			"func (a " + typ + ") Value() (driver.Value, error) { return int64(a), nil }\n"
	}
	files := map[string]string{
		"go.mod": "module aliascheck\n\ngo 1.27\n\n" +
			"require github.com/Piechutowski/volt v0.0.0\n\n" +
			"replace github.com/Piechutowski/volt => " + repoRoot + "\n",
		"cash/money.go":    scanner("money", "Amount"),
		"levels/v2/lvl.go": scanner("levels", "Level"),
	}
	if err := os.MkdirAll(filepath.Join(dir, "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	src := `Type cash.Amount int from "aliascheck/cash"
Type v2.Level int from "aliascheck/levels/v2"

Table orders {
  id    int         [pk, increment]
  total cash.Amount
  tier  v2.Level    [null]
}
`
	f, diags := parser.ParseFile("schema.volt", src)
	info, semDiags := check.File(f)
	if diags = append(diags, semDiags...); diag.HasErrors(diags) {
		t.Fatalf("schema must check clean: %v", diags)
	}
	opts := Options{Package: "models", Source: "schema.volt"}
	validators := func(f *ast.File, info *check.Info, opts Options) ([]byte, error) {
		return GenerateValidators(f, info, nil, opts)
	}
	for name, gen := range map[string]func(*ast.File, *check.Info, Options) ([]byte, error){
		"nao_models.go":   Generate,
		"nao_queries.go":  GenerateQueries,
		"nao_dyn.go":      GenerateDyn,
		"nao_validate.go": validators,
	} {
		code, err := gen(f, info, opts)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if name == "nao_models.go" && (!bytes.Contains(code, []byte(`cash "aliascheck/cash"`)) || !bytes.Contains(code, []byte(`v2 "aliascheck/levels/v2"`))) {
			t.Errorf("%s does not import the Type packages under their aliases:\n%s", name, code)
		}
		if err := os.WriteFile(filepath.Join(dir, "models", name), code, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated code does not compile:\n%s", out)
	}
}

func TestGeneratedHeader(t *testing.T) {
	re := regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)
	for _, golden := range goldenFiles(t) {
		src, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		first := strings.SplitN(string(src), "\n", 2)[0]
		if !re.MatchString(first) {
			t.Errorf("%s first line %q does not match the generated-code convention", golden, first)
		}
	}
}

func goldenFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "*.go.golden"))
	if err != nil || len(files) == 0 {
		t.Fatal("no golden files; run 'go test ./gen/... -update' first")
	}
	return files
}

func TestGenerationErrors(t *testing.T) {
	cases := []struct {
		name, src, wantErr string
	}{
		{
			name:    "field collision",
			src:     "Table t { user_id int\n user__id int }",
			wantErr: "both map to Go field",
		},
		{
			name:    "struct collision",
			src:     "Table user_roles { id int }\nTable user__roles { id int }",
			wantErr: "both map to Go type",
		},
		{
			name:    "unusable name",
			src:     "Table t { __ int }",
			wantErr: "no characters usable",
		},
		{
			name:    "package-scope name collision",
			src:     "Table clashes { id int [pk]\n new int\n column int\n params int }",
			wantErr: "both need the Go name",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, diags := parser.ParseFile("t", tc.src)
			info, semDiags := check.File(f)
			if diag.HasErrors(append(diags, semDiags...)) {
				t.Fatalf("input unexpectedly invalid: %v %v", diags, semDiags)
			}
			_, err := Generate(f, info, Options{Package: "models", Source: "t"})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestGoNames(t *testing.T) {
	cases := []struct{ in, want string }{
		{"user_id", "UserID"},
		{"api_key", "APIKey"},
		{"http_status_url", "HTTPStatusURL"},
		{"uuid", "UUID"},
		{"full name", "FullName"},
		{"_2fa_codes", "X2faCodes"},
		{"created_at", "CreatedAt"},
		{"id", "ID"},
		{"żółć", "Żółć"},
	}
	for _, tc := range cases {
		got, err := goName(tc.in)
		if err != nil {
			t.Errorf("goName(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("goName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if _, err := goName("__"); err == nil {
		t.Error("goName(__): want error")
	}
}
