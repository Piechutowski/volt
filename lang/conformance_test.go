package lang

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/diag"
)

var update = flag.Bool("update", false, "rewrite the goldens of the invalid corpus")

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
		path := filepath.Join(dir, e.Name())
		if !e.IsDir() && !hasPackageClause(t, path) {
			continue
		}
		out = append(out, path)
	}
	return out
}

func hasPackageClause(t *testing.T, path string) bool {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return pkgClauseRE.Match(src)
}

func corpusRun(t *testing.T, path string) []diag.Diagnostic {
	t.Helper()
	diags, _ := corpusRunAt(t, path)
	return diags
}

func corpusRunAt(t *testing.T, path string) ([]diag.Diagnostic, string) {
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
	return Check(pr), root
}

func TestConformanceInvalidDiagnosticsPinned(t *testing.T) {
	var b strings.Builder
	for _, path := range corpusEntries(t, "invalid") {
		diags, root := corpusRunAt(t, path)
		diag.Sort(diags)
		fmt.Fprintf(&b, "## %s\n", filepath.Base(path))
		for _, d := range diags {
			b.WriteString(strings.ReplaceAll(d.String(), root+string(filepath.Separator), "") + "\n")
		}
		b.WriteString("\n")
	}
	goldenCompare(t, filepath.Join("conformance", "invalid_volt.golden"), b.String(), *update)
}

func goldenCompare(t *testing.T, path, got string, update bool) {
	t.Helper()
	if update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to write it)", err)
	}
	if string(want) == got {
		return
	}
	wl, gl := strings.Split(string(want), "\n"), strings.Split(got, "\n")
	for i := range gl {
		if i >= len(wl) || wl[i] != gl[i] {
			w := "<end>"
			if i < len(wl) {
				w = wl[i]
			}
			t.Fatalf("%s differs at line %d:\n--- golden\n%s\n--- got\n%s\n(run with -update after reading the diff)", path, i+1, w, gl[i])
		}
	}
	t.Fatalf("%s: got is a prefix of the golden (run with -update after reading the diff)", path)
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

func TestConformanceTagged(t *testing.T) {
	spec := specText(t)
	partII := map[string]bool{}
	start, end := strings.Index(spec, "\n# Part II"), strings.Index(spec, "\n# Appendices")
	for _, m := range specHeadings(spec[start:end]) {
		partII[anchorOf(strings.TrimSpace(m[2]))] = true
	}
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
				for _, m := range specCiteRE.FindAllStringSubmatch(string(src), -1) {
					if partII[m[1]] {
						tagged = true
					}
				}
			}
			if !tagged {
				t.Errorf("%s carries no '// spec: §…' tag naming a Part II section", path)
			}
		}
	}
}
