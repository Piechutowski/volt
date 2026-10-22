package sqlite

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

var update = flag.Bool("update", false, "rewrite golden files")

func generate(t *testing.T, path string) []byte {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, diags := parser.ParseFile(filepath.Base(path), string(src))
	info, semDiags := check.File(f)
	diags = append(diags, semDiags...)
	if diag.HasErrors(diags) {
		t.Fatalf("test input must check clean: %v", diags)
	}
	out, err := Generate(f, info, Options{Source: filepath.Base(path)})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return out
}

func TestGolden(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "testdata", "*.volt"))
	if err != nil {
		t.Fatal(err)
	}
	own, err := filepath.Glob(filepath.Join("testdata", "*.volt"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, own...)
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("no shared corpus schemas in gen/testdata")
	}
	for _, schema := range files {
		schema := schema
		t.Run(filepath.Base(schema), func(t *testing.T) {
			got := generate(t, schema)
			golden := filepath.Join("testdata", strings.TrimSuffix(filepath.Base(schema), ".volt")+".sql.golden")
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
		})
	}
}

func TestGoldenExecutes(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	const driver = `
import sqlite3, sys
con = sqlite3.connect(":memory:")
con.execute("PRAGMA foreign_keys = ON")
import datetime, uuid
def _now(): return datetime.datetime.now().isoformat(sep=" ")
con.create_function("now", 0, _now, deterministic=True)
con.create_function("getdate", 0, _now, deterministic=True)
con.create_function("uuid_generate_v4", 0, lambda: str(uuid.uuid4()), deterministic=True)
con.executescript(sys.stdin.read())
con.execute("PRAGMA foreign_key_check").fetchall() and sys.exit("foreign key violations")
`
	for _, golden := range goldenFiles(t) {
		golden := golden
		t.Run(filepath.Base(golden), func(t *testing.T) {
			src, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("python3", "-c", driver)
			cmd.Stdin = bytes.NewReader(src)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("SQLite rejected %s: %v\n%s", golden, err, out)
			}
		})
	}
}

func TestGeneratedHeader(t *testing.T) {
	re := regexp.MustCompile(`^-- Code generated .* DO NOT EDIT\.$`)
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
	files, err := filepath.Glob(filepath.Join("testdata", "*.sql.golden"))
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
			name:    "two indexes of one table derive one name",
			src:     "Table users {\n  id int [pk]\n  email string [unique]\n  Indexes {\n    email [where: `email != ''`]\n  }\n}",
			wantErr: `duplicate index name "idx_users_email"`,
		},
		{
			name:    "two tables derive one index name",
			src:     "Table post {\n  id int [pk]\n  tag_slug string [unique]\n}\nTable post_tag [model: \"Label\"] {\n  id int [pk]\n  slug string [index]\n}",
			wantErr: `duplicate index name "idx_post_tag_slug", an index of table post`,
		},
		{
			name:    "an index takes a table's name in another case",
			src:     "Table posts {\n  id int [pk]\n  a int\n  b int\n  Indexes {\n    (a, b) [name: \"Pages\"]\n  }\n}\nTable pages {\n  id int [pk]\n}",
			wantErr: `duplicate index name "Pages", the name of table pages`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, diags := parser.ParseFile("t", tc.src)
			info, semDiags := check.File(f)
			if diag.HasErrors(diags) {
				t.Fatalf("input unexpectedly unparsable: %v", diags)
			}
			if !diag.HasErrors(semDiags) {
				t.Fatalf("the checker reports two indexes of one name first (§indexes); got %v", semDiags)
			}
			_, err := Generate(f, info, Options{Source: "t"})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestQuoting(t *testing.T) {
	cases := []struct{ in, want string }{
		{"users", "users"},
		{"order", `"order"`},
		{"full name", `"full name"`},
		{"2fa_codes", `"2fa_codes"`},
		{"Users", `"Users"`},
		{`we"ird`, `"we""ird"`},
	}
	for _, tc := range cases {
		if got := identQuote(tc.in); got != tc.want {
			t.Errorf("identQuote(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
	if got := stringQuote("it's"); got != "'it''s'" {
		t.Errorf("stringQuote = %s", got)
	}
}
