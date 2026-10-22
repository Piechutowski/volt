package lsp

import (
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

const typeSchema = "package db\n\nType money.Money int from \"example.com/app/money\"\nType Slug string\n\nTable products {\n\tid int [pk, increment]\n\tslug Slug\n\tprice money.Money\n}\n"

func TestTypeHoverAndDefinition(t *testing.T) {
	d := NewDocument("file:///lone.volt", typeSchema)
	h := d.Hover(posOf(t, typeSchema, "Money\n}", 0))
	if h == nil {
		t.Fatal("no hover for the declared type reference")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	if !strings.Contains(md, "Type money.Money int from \"example.com/app/money\"") {
		t.Errorf("hover does not show the declaration:\n%s", md)
	}
	loc := d.Definition(posOf(t, typeSchema, "Slug\n", 0))
	if loc == nil {
		t.Fatal("no definition for the declared type reference")
	}
	if want := posOf(t, typeSchema, "Slug string", 0); loc.Range.Start != want {
		t.Errorf("definition = %v, want %v", loc.Range.Start, want)
	}
}
