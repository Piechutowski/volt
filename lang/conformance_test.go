package lang

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/diag"
)

// The Volt half of the conformance corpus is the executable surface of
// docs/spec.md §V: snippets under valid/ MUST check clean, snippets
// under invalid/ MUST be rejected. A snippet is either one .volt file
// (run as a single-package project, the package directory named from
// its package clause) or a directory containing a complete project
// with go.mod (D62). The .dbml entries sharing the tree are the schema
// half, exercised by lang/check's corpus test.

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
		if strings.HasSuffix(e.Name(), ".dbml") {
			continue // the schema half; lang/check's corpus test runs it
		}
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
			// A `// want: <substring>` line pins the reason: the snippet
			// must be rejected for the rule its comment names, not by
			// accident.
			if want := corpusWant(t, path); want != "" {
				found := false
				for _, d := range diags {
					if strings.Contains(d.Msg, want) {
						found = true
					}
				}
				if !found {
					t.Errorf("rejected, but not for the pinned reason %q; diagnostics: %v", want, diags)
				}
			}
		})
	}
}

var wantRE = regexp.MustCompile(`(?m)^// want: (.+)$`)

// corpusWant returns the snippet's `// want:` pin, "" when absent. For
// a directory snippet the first .volt file (name order) carries it.
func corpusWant(t *testing.T, path string) string {
	t.Helper()
	file := path
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		var files []string
		filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".volt") {
				files = append(files, p)
			}
			return nil
		})
		if len(files) == 0 {
			return ""
		}
		sort.Strings(files)
		file = files[0]
	}
	src, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	if m := wantRE.FindSubmatch(src); m != nil {
		return strings.TrimSpace(string(m[1]))
	}
	return ""
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
