package lsp

import (
	"os/exec"
	"path/filepath"
	"testing"
)

const goTypeSchema = "package db\n\n" +
	"type units.Commit string from \"g/units\"\n" +
	"type tm.Duration int from \"time\"\n" +
	"type Phase string\n\n" +
	"table runs {\n\tid int [pk]\n\tcommit units.Commit\n\tphase Phase\n\ttook tm.Duration\n}\n"

const goTypeUnits = "package units\n\n// Commit is a git commit id.\ntype Commit string\n"

// TestTypeDeclarationGoesToGo proves go to definition on the name of a
// type declaration lands on the Go type it admits (§types): a qualified
// name in the package its from path imports, the project's own and the
// standard library's alike, on either part of the name, and a bare name
// in the data package's own Go files.
func TestTypeDeclarationGoesToGo(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("a from path resolves through the go tool, which is not on PATH:", err)
	}
	root := voltProject(t, map[string]string{
		"go.mod":         "module g\n",
		"db/schema.volt": goTypeSchema,
		"db/db.go":       "package db\n\n// Phase is a measured phase.\ntype Phase string\n",
		"units/units.go": goTypeUnits,
	})
	d := NewDocument("file://"+filepath.Join(root, "db", "schema.volt"), goTypeSchema)
	if diags := d.LSPDiagnostics(); len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	for _, c := range []struct {
		needle string
		file   string
		line   int
	}{
		{"Commit string", filepath.Join(root, "units", "units.go"), 3},
		{"units.Commit string", filepath.Join(root, "units", "units.go"), 3},
		{"Phase string", filepath.Join(root, "db", "db.go"), 3},
	} {
		loc := d.Definition(posOf(t, goTypeSchema, c.needle, 0))
		if loc == nil || loc.URI != "file://"+c.file || int(loc.Range.Start.Line) != c.line || loc.Range.Start.Character != 5 {
			t.Errorf("definition at %q = %+v, want %s line %d", c.needle, loc, c.file, c.line+1)
		}
	}
	loc := d.Definition(posOf(t, goTypeSchema, "Duration int", 0))
	if loc == nil || filepath.Base(string(loc.URI)) == "schema.volt" {
		t.Errorf("a type of the standard library resolved to %+v", loc)
	}
	loc = d.Definition(posOf(t, goTypeSchema, "Commit\n", 0))
	if loc == nil || loc.Range.Start != posOf(t, goTypeSchema, "Commit string", 0) {
		t.Errorf("a column's declared type resolved to %+v, want the type declaration", loc)
	}
}
