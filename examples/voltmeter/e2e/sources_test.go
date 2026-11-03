package e2e

import (
	"bytes"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/scanner"
	vtoken "github.com/Piechutowski/volt/lang/token"
)

func voltSources(t *testing.T) map[string]string {
	t.Helper()
	mod := moduleDir(t)
	files := map[string]string{}
	err := filepath.WalkDir(mod, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".volt" {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(mod, p)
		files[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 4 {
		t.Fatalf("found %d .volt files", len(files))
	}
	return files
}

var commentRE = regexp.MustCompile(`(?s)//[^\n]*|/\*.*?\*/`)

// TestSourcesSpeakTheLexicon scans every .volt file of Voltmeter with the
// front end's scanner and proves the application is written in the whole
// lexicon: no lexical error; line and block comments that leave nothing
// but whitespace between tokens; identifiers, numbers, expression literals,
// single-line strings with an escape and a multi-line string whose
// indentation is stripped; the literal words false and null; the longest match of >=;
// and every element starting in the first column, the package clause
// first (spec §lexical-structure, §program-structure).
func TestSourcesSpeakTheLexicon(t *testing.T) {
	kinds := map[vtoken.Kind]int{}
	literals := map[string]bool{}
	var lineComments, blockComments, escapes, multis int
	for name, src := range voltSources(t) {
		toks, errs := scanner.Scan(name, src)
		for _, e := range errs {
			t.Errorf("%s: %v", name, e)
		}
		end, first := 0, true
		for _, tk := range toks {
			gap := src[end:tk.Pos.Offset()]
			for _, c := range commentRE.FindAllString(gap, -1) {
				if strings.HasPrefix(c, "//") {
					lineComments++
				} else {
					blockComments++
				}
			}
			if rest := commentRE.ReplaceAllString(gap, ""); strings.TrimSpace(rest) != "" {
				t.Errorf("%s: %q between tokens is neither whitespace nor a comment", name, rest)
			}
			if tk.Kind == vtoken.EOF {
				break
			}
			end = tk.Pos.Offset() + len(tk.Text())
			kinds[tk.Kind]++
			if tk.DeclStart {
				if tk.Pos.Column() != 1 {
					t.Errorf("%s: element start %v is not in the first column", name, tk.Pos)
				}
				if first && tk.Val != "package" {
					t.Errorf("%s: the first element is %q, not the package clause", name, tk.Val)
				}
				first = false
			}
			switch tk.Kind {
			case vtoken.IDENT:
				if tk.Val == "false" || tk.Val == "null" {
					literals[tk.Val] = true
				}
			case vtoken.STRING:
				if tk.Multi {
					multis++
					if !strings.HasPrefix(tk.Text(), `"""`) || strings.HasPrefix(tk.Val, " ") || strings.HasPrefix(tk.Val, "\n") || strings.HasSuffix(tk.Val, "\n") || strings.Contains(tk.Val, "\n ") {
						t.Errorf("%s: the multi-line string %q keeps its indentation", name, tk.Val)
					}
				}
				if strings.Contains(tk.Text(), `\"`) {
					escapes++
					if strings.Contains(tk.Val, `\`) || !strings.Contains(tk.Val, `"`) {
						t.Errorf("%s: the escape in %s is not interpreted: %q", name, tk.Text(), tk.Val)
					}
				}
			case vtoken.GE:
				if tk.Text() != ">=" {
					t.Errorf("%s: >= scanned as %q", name, tk.Text())
				}
			}
		}
	}
	if lineComments == 0 || blockComments == 0 {
		t.Errorf("line comments %d, block comments %d: the sources should carry both forms", lineComments, blockComments)
	}
	if multis == 0 || escapes == 0 {
		t.Errorf("multi-line strings %d, escaped strings %d", multis, escapes)
	}
	for _, w := range []string{"false", "null"} {
		if !literals[w] {
			t.Errorf("no %s literal in the sources", w)
		}
	}
	for _, k := range []vtoken.Kind{
		vtoken.IDENT, vtoken.STRING, vtoken.NUMBER, vtoken.FUNCEXPR,
		vtoken.LBRACE, vtoken.RBRACE, vtoken.LBRACKET, vtoken.RBRACKET, vtoken.LPAREN, vtoken.RPAREN,
		vtoken.COMMA, vtoken.COLON, vtoken.DOT, vtoken.TILDE, vtoken.STAR, vtoken.SLASH,
		vtoken.LT, vtoken.GT, vtoken.BACKSLASH, vtoken.EQ, vtoken.GE,
	} {
		if kinds[k] == 0 {
			t.Errorf("no %s token in the sources", k)
		}
	}
}

type generated struct {
	dir   string
	files []string
	may   func(path string) bool
}

// TestGeneratedFilesKeepTheLayers proves Voltmeter's generated files are
// the contract's set, each under the one header and gofmt-stable, and
// that each layer imports only what it may: the data files the standard
// library without net/http, nao/rt and the declared types' package, with
// database/sql only where the declared-type assertions stand; the route
// files the standard library, the root volt package, the runner and the
// data package; the client the same without the runner (spec
// §data-and-network-layers, §generation-contract-informative).
func TestGeneratedFilesKeepTheLayers(t *testing.T) {
	const root = "github.com/Piechutowski/volt"
	const self = root + "/examples/voltmeter"
	std := func(p string) bool { return !strings.Contains(strings.SplitN(p, "/", 2)[0], ".") && p != "net/http" }
	mod := moduleDir(t)
	for _, g := range []generated{
		{"db", []string{"volt_dyn.go", "volt_migrations.go", "volt_models.go", "volt_queries.go", "volt_schema.sql", "volt_selects.go", "volt_validate.go"},
			func(p string) bool { return std(p) || p == root+"/nao/rt" || p == self+"/units" }},
		{"app", []string{"volt_handlers.go", "volt_paths.go", "volt_router.go", "volt_routes.go"},
			func(p string) bool {
				return std(p) || p == "net/http" || p == root || p == root+"/nao/run" || p == self+"/db"
			}},
		{"app/client", []string{"volt_client.go"},
			func(p string) bool { return std(p) || p == "net/http" || p == root || p == self+"/db" }},
	} {
		pkg := filepath.Base(g.dir)
		if g.dir == "app/client" {
			pkg = "app"
		}
		entries, err := os.ReadDir(filepath.Join(mod, g.dir))
		if err != nil {
			t.Fatal(err)
		}
		var found []string
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "volt_") {
				found = append(found, e.Name())
			}
		}
		if !slices.Equal(found, g.files) {
			t.Errorf("%s holds %v, the contract's set is %v", g.dir, found, g.files)
		}
		for _, name := range g.files {
			path := filepath.Join(mod, g.dir, name)
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			header := "// Code generated by volt gen from package " + pkg + ". DO NOT EDIT.\n"
			if filepath.Ext(name) == ".sql" {
				header = "-- Code generated by volt gen from package " + pkg + ". DO NOT EDIT.\n"
			}
			if !bytes.HasPrefix(src, []byte(header)) {
				t.Errorf("%s/%s does not start with %q", g.dir, name, header)
			}
			if filepath.Ext(name) != ".go" {
				continue
			}
			if formatted, err := format.Source(src); err != nil || !bytes.Equal(formatted, src) {
				t.Errorf("%s/%s is not gofmt-stable: %v", g.dir, name, err)
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, src, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, im := range f.Imports {
				p, _ := strconv.Unquote(im.Path.Value)
				if !g.may(p) {
					t.Errorf("%s/%s imports %s, outside its layer", g.dir, name, p)
				}
				if strings.HasPrefix(p, "database/sql") && !bytes.Contains(src, []byte("_ sql.Scanner")) {
					t.Errorf("%s/%s imports %s for something other than the declared-type assertions", g.dir, name, p)
				}
			}
		}
	}
}
