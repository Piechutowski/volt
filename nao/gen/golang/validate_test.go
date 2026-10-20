package golang

import (
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/diag"
	"github.com/Piechutowski/volt/lang/parser"
)

// TestValidateRequiredDeclaredType pins the required rule over a declared
// type (spec §changesets rule 9): the empty value is read through the
// type's driver.Valuer by rt.Blank, since the generator cannot know the
// type's zero value.
func TestValidateRequiredDeclaredType(t *testing.T) {
	src := "package shop\n\nType Cents int\n\nTable posts {\n  id int [pk, increment]\n  price Cents [not null, required]\n  title string [not null, required]\n}\n"
	f, diags := parser.ParseFile("shop.volt", src)
	info, semDiags := check.File(f)
	if diag.HasErrors(append(diags, semDiags...)) {
		t.Fatalf("input unexpectedly invalid: %v %v", diags, semDiags)
	}
	out, err := GenerateValidators(f, info, nil, Options{Package: "shop", Source: "shop.volt"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"\tif rt.Blank(v.Price) {\n",
		`Check: "required", Columns: []string{"price"}, Message: "price is required"`,
		"\tif v.Title == \"\" {\n",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("nao_validate.go lacks %q:\n%s", want, out)
		}
	}
}
