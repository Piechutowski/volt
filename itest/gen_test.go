package itest

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Piechutowski/volt/gen/router"
	"github.com/Piechutowski/volt/lang"
	"github.com/Piechutowski/volt/lang/diag"
)

// TestFixtureNotDrifted regenerates the fixture's router files
// in-memory and byte-compares them with the committed ones: the proof
// suite always exercises current generator output. Refresh with
// 'go run ./cmd/volt gen ./itest/blog/app' after intentional changes.
func TestFixtureNotDrifted(t *testing.T) {
	// The fixture is a package of this repository's Go module (§V1.1),
	// loaded the way 'volt gen ./itest/blog/app' loads it.
	root, err := lang.FindRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	pr, err := lang.LoadDirs(root, []string{filepath.Join("blog", "app")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	diags := lang.Check(pr)
	if diag.HasErrors(diags) {
		t.Fatalf("fixture has errors: %v", diags)
	}
	pkg := pr.PackageAt(filepath.Join("blog", "app"))
	if pkg == nil {
		t.Fatal("blog/app not loaded")
	}
	files, err := router.Generate(pkg, router.Options{Source: "package " + pkg.Path})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join("blog", "app", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s drifted from generator output; re-run volt gen ./itest/blog/app", name)
		}
	}
}
