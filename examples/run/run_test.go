package main

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGeneratedFilesCurrent regenerates every example with the volt
// binary of this checkout, in a copy of the module, and proves the
// committed generated files are byte-identical, none is missing and
// none is left over, so what an example shows beside its .volt files is
// what volt gen writes. On failure: go run ./examples/run NAME.
func TestGeneratedFilesCurrent(t *testing.T) {
	dir, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	volt := filepath.Join(t.TempDir(), "volt")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", volt, "./cmd/volt")
	build.Dir = filepath.Dir(dir)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building volt: %v\n%s", err, out)
	}
	names, err := examples(dir)
	if err != nil || len(names) != 5 {
		t.Fatalf("examples %v, %v", names, err)
	}
	for _, name := range names {
		mod := t.TempDir()
		copyTree(t, filepath.Join(dir, name), filepath.Join(mod, name))
		copyFile(t, filepath.Join(dir, "go.mod"), filepath.Join(mod, "go.mod"))
		gen := exec.Command(volt, "gen", "--sql", "./...")
		gen.Dir = filepath.Join(mod, name)
		if out, err := gen.CombinedOutput(); err != nil {
			t.Errorf("%s: volt gen: %v\n%s", name, err, out)
			continue
		}
		want := generated(t, filepath.Join(mod, name))
		have := generated(t, filepath.Join(dir, name))
		for rel, data := range want {
			if !bytes.Equal(have[rel], data) {
				t.Errorf("%s/%s differs from what volt gen writes; run go run ./examples/run %s", name, rel, name)
			}
		}
		for rel := range have {
			if _, ok := want[rel]; !ok {
				t.Errorf("%s/%s is no longer generated", name, rel)
			}
		}
	}
}

func generated(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasPrefix(d.Name(), "volt_") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		out[rel] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		if strings.HasPrefix(d.Name(), "volt_") {
			return nil
		}
		copyFile(t, path, filepath.Join(to, rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	data, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
