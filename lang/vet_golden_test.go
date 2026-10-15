package lang

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/diag"
)

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
