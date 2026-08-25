package lang

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/nao/edbml/diag"
)

// The conformance corpus is the executable surface of SPEC.md §V:
// snippets under valid/ MUST check clean, snippets under invalid/ MUST
// be rejected. A snippet is either one .volt file (run as a
// single-package project, the package directory named from its package
// clause) or a directory containing a complete project with volt.mod.

var pkgClauseRE = regexp.MustCompile(`(?im)^package\s+(\w+)`)

func corpusEntries(t *testing.T, kind string) []string {
	t.Helper()
	dir := filepath.Join("conformance", "snippets", kind)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, filepath.Join(dir, e.Name()))
	}
	return out
}

// corpusRun materializes one snippet as a project and returns its
// diagnostics.
func corpusRun(t *testing.T, path string) []diag.Diagnostic {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if st.IsDir() {
		if err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(path, p)
			dst := filepath.Join(root, rel)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			return os.WriteFile(dst, src, 0o644)
		}); err != nil {
			t.Fatal(err)
		}
	} else {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		pkgName := "app"
		if m := pkgClauseRE.FindSubmatch(src); m != nil {
			pkgName = strings.ToLower(string(m[1]))
		}
		if err := os.WriteFile(filepath.Join(root, ModFile), []byte("module corpus\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(root, pkgName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(path)), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pr, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return Check(pr)
}

func TestConformanceValid(t *testing.T) {
	for _, path := range corpusEntries(t, "valid") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			diags := corpusRun(t, path)
			if diag.HasErrors(diags) {
				t.Errorf("valid snippet rejected:\n%v", diags)
			}
		})
	}
}

func TestConformanceInvalid(t *testing.T) {
	for _, path := range corpusEntries(t, "invalid") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			diags := corpusRun(t, path)
			if !diag.HasErrors(diags) {
				t.Errorf("invalid snippet accepted")
			}
		})
	}
}

// TestConformanceTagged pins the corpus to the spec: every snippet
// cites the section it exercises.
func TestConformanceTagged(t *testing.T) {
	for _, kind := range []string{"valid", "invalid"} {
		for _, path := range corpusEntries(t, kind) {
			tagged := false
			st, _ := os.Stat(path)
			var files []string
			if st.IsDir() {
				filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
					if err == nil && !d.IsDir() && strings.HasSuffix(p, ".volt") {
						files = append(files, p)
					}
					return nil
				})
			} else {
				files = []string{path}
			}
			for _, f := range files {
				src, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(src), "// spec: §V") {
					tagged = true
				}
			}
			if !tagged {
				t.Errorf("%s carries no '// spec: §V…' tag", path)
			}
		}
	}
}
