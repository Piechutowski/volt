package lang

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/diag"
)

// The complete lint verdict of one project is a golden. Every rule of
// docs/lint.md fires at least once in testdata/vet, and the fixture
// holds the shapes a per-declaration vet must reproduce exactly: one
// rule speaking for one cause across two files of a package, a name
// minted by three declarations, an enum declared after the table whose
// handle it collides with, a partial's inline ref carried into every
// table that injects it, a duplicate table, a pipeline no scope pipes
// through. The golden pins position, code and message of every warning
// and how many times each speaks, so a change to how vet is computed
// shows its every effect here (D104).
func TestVetGolden(t *testing.T) {
	root := filepath.Join("testdata", "vet")
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	pr, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	render := func(ds []diag.Diagnostic) string {
		var b strings.Builder
		for _, d := range ds {
			b.WriteString(strings.TrimPrefix(d.String(), abs+string(filepath.Separator)))
			b.WriteByte('\n')
		}
		return b.String()
	}
	var b strings.Builder
	b.WriteString("check:\n")
	b.WriteString(render(Check(pr)))
	b.WriteString("vet:\n")
	b.WriteString(render(Vet(pr)))
	goldenCompare(t, filepath.Join("testdata", "vet.golden"), b.String(), *update)
}
