package align

import (
	"bytes"
	"fmt"
	"go/format"
	"math/rand"
	"strings"
	"testing"
)

// TestBlockMatchesGofmt renders random struct, const and var blocks and
// proves gofmt leaves them unchanged: the layout is gofmt's, not an
// approximation of it.
func TestBlockMatchesGofmt(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	word := func(min, max int) string {
		n := min + rng.Intn(max-min+1)
		var b strings.Builder
		b.WriteByte('A' + byte(rng.Intn(26)))
		for i := 1; i < n; i++ {
			b.WriteByte('a' + byte(rng.Intn(26)))
		}
		return b.String()
	}
	types := []string{"int32", "string", "rt.Null[int64]", "time.Time", "[]byte", "*data01.Queries", "Ünïcode"}
	for iter := 0; iter < 300; iter++ {
		var src strings.Builder
		src.WriteString("package p\n\n")

		// struct: fields with and without tags, doc comments, blank lines,
		// an embedded field, trailing comments
		var fields Block
		if rng.Intn(3) == 0 {
			fields.Row("volt.Client")
		}
		for i := 0; i < 1+rng.Intn(8); i++ {
			switch rng.Intn(6) {
			case 0:
				fields.Line("// " + word(3, 20))
			case 1:
				if fields.Len() > 0 {
					fields.Line("") // gofmt drops a blank line right after the brace
				}
			}
			cells := []string{word(1, 12), types[rng.Intn(len(types))]}
			if rng.Intn(2) == 0 {
				cells = append(cells, fmt.Sprintf("`db:%q`", strings.ToLower(cells[0])))
			}
			if rng.Intn(4) == 0 {
				cells = append(cells, "// "+word(2, 10))
			}
			fields.Row(cells...)
		}
		src.WriteString("type S struct {\n")
		fields.WriteTo(&src, "\t")
		src.WriteString("}\n\n")

		// typed const block with comments between values
		var consts Block
		for i := 0; i < 1+rng.Intn(5); i++ {
			if rng.Intn(3) == 0 {
				consts.Line("// " + word(3, 12))
			}
			consts.Row("E"+word(2, 10), "EKind", fmt.Sprintf("= %q", word(1, 6)))
		}
		src.WriteString("type EKind string\n\nconst (\n")
		consts.WriteTo(&src, "\t")
		src.WriteString(")\n\n")

		// var block with = alignment
		var vars Block
		for i := 0; i < 1+rng.Intn(5); i++ {
			if rng.Intn(4) == 0 && vars.Len() > 0 {
				vars.Line("")
			}
			vars.Row(word(2, 14), fmt.Sprintf("= rt.Column[S, int32]{Name: %q}", word(1, 5)))
		}
		src.WriteString("var (\n")
		vars.WriteTo(&src, "\t")
		src.WriteString(")\n")

		got := Finish(src.String())
		want, err := format.Source(got)
		if err != nil {
			t.Fatalf("iteration %d: %v\n%s", iter, err, got)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("iteration %d: block layout differs from gofmt\n--- ours\n%s\n--- gofmt\n%s", iter, got, want)
		}
	}
}

func TestFinish(t *testing.T) {
	for _, in := range []string{"x", "x\n", "x\n\n\n"} {
		if got := string(Finish(in)); got != "x\n" {
			t.Errorf("Finish(%q) = %q", in, got)
		}
	}
	if got := string(Finish("a\n\n\n\nb\n\n")); got != "a\n\nb\n" {
		t.Errorf("Finish did not collapse blank lines: %q", got)
	}
}
