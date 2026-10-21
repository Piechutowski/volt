package lsp

import (
	"slices"
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/parser"
	"github.com/Piechutowski/volt/lang/token"
)

const sample = `Type Status string

TablePartial base {
  id int [pk]
}

Table users [note: 'people'] {
  ~base
  name string [unique]
  status Status [null, default: 'active']
}

Table orders {
  id int [pk]
  user_id int [null]
  owner_name string [null, ref: users.name]
}

Ref: orders.user_id > users.id

Group g {
  users
  orders
}
`

func doc(t *testing.T, text string) *Document {
	t.Helper()
	return NewDocument("file:///test.volt", text)
}

func posOf(t *testing.T, text, needle string, occurrence int) protocol.Position {
	t.Helper()
	off := -1
	for i := 0; i <= occurrence; i++ {
		next := strings.Index(text[off+1:], needle)
		if next < 0 {
			t.Fatalf("needle %q occurrence %d not found", needle, occurrence)
		}
		off += 1 + next
	}
	line := strings.Count(text[:off], "\n")
	col := off - (strings.LastIndex(text[:off], "\n") + 1)
	return protocol.Position{Line: protocol.UInteger(line), Character: protocol.UInteger(col)}
}

func TestCleanSampleHasNoDiagnostics(t *testing.T) {
	d := doc(t, sample)
	if len(d.Diags) != 0 {
		t.Fatalf("expected clean sample, got %v", d.Diags)
	}
}

func TestDiagnosticsOnError(t *testing.T) {
	d := doc(t, "Table t {\n  id int [pk\n}\n")
	got := d.LSPDiagnostics()
	if len(got) == 0 {
		t.Fatal("expected diagnostics for unclosed settings")
	}
	if *got[0].Severity != protocol.DiagnosticSeverityError {
		t.Errorf("expected error severity, got %v", *got[0].Severity)
	}
}

func TestVetWarningsSurface(t *testing.T) {
	d := doc(t, "Table t {\n  id int [pk]\n  indexes {\n  }\n}\n")
	found := false
	for _, dg := range d.Diags {
		if strings.HasPrefix(dg.Code, "vet/") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a vet warning, got %v", d.Diags)
	}
}

func TestModelnameAnalyzerExcluded(t *testing.T) {
	d := doc(t, "Table status {\n  id int [pk]\n}\nTable other {\n  id int [pk]\n  s int [null, ref: status.id]\n}\n")
	for _, dg := range d.Diags {
		if strings.Contains(dg.Code, "modelname") {
			t.Fatalf("modelname analyzer should be excluded, got %v", dg)
		}
	}
}

func TestDefinitionFromRefEndpoint(t *testing.T) {
	d := doc(t, sample)
	pos := posOf(t, sample, "> users.id", 0)
	pos.Character += 2
	loc := d.Definition(pos)
	if loc == nil {
		t.Fatal("no definition for ref endpoint table")
	}
	want := posOf(t, sample, "Table users [", 0)
	if loc.Range.Start.Line != want.Line+0 {
		t.Errorf("definition line = %d, want %d", loc.Range.Start.Line, want.Line)
	}
}

func TestDefinitionFromInlineRef(t *testing.T) {
	d := doc(t, sample)
	pos := posOf(t, sample, "ref: users.name", 0)
	pos.Character += protocol.UInteger(len("ref: "))
	loc := d.Definition(pos)
	if loc == nil {
		t.Fatal("no definition from an inline ref")
	}
	declPos := posOf(t, sample, "Table users [", 0)
	if loc.Range.Start.Line != declPos.Line {
		t.Errorf("inline ref definition line = %d, want %d", loc.Range.Start.Line, declPos.Line)
	}
}

func TestDefinitionPartialInjection(t *testing.T) {
	d := doc(t, sample)
	pos := posOf(t, sample, "~base", 0)
	pos.Character += 2
	loc := d.Definition(pos)
	if loc == nil {
		t.Fatal("no definition for ~base")
	}
	want := posOf(t, sample, "TablePartial base", 0)
	if loc.Range.Start.Line != want.Line {
		t.Errorf("partial definition line = %d, want %d", loc.Range.Start.Line, want.Line)
	}
}

func TestDefinitionDeclaredType(t *testing.T) {
	d := doc(t, sample)
	pos := posOf(t, sample, "status Status", 0)
	pos.Character += protocol.UInteger(len("status "))
	loc := d.Definition(pos)
	if loc == nil {
		t.Fatal("no definition for declared type reference")
	}
	if loc.Range.Start.Line != 0 {
		t.Errorf("Type definition line = %d, want 0", loc.Range.Start.Line)
	}
}

func TestReferencesForTable(t *testing.T) {
	d := doc(t, sample)
	pos := posOf(t, sample, "Table users [", 0)
	pos.Character += protocol.UInteger(len("Table "))
	refs := d.References(pos, true)
	if len(refs) < 3 {
		t.Fatalf("expected >= 3 references for users (declaration, inline ref, Ref endpoint; a Group member is a reference in a package, which TestGroupRenameFollowsSetUses covers), got %d: %v", len(refs), refs)
	}
}

func TestRenameColumnFromPartial(t *testing.T) {
	d := doc(t, sample)
	pos := posOf(t, sample, "id int [pk]", 0)
	edit, err := d.Rename(pos, "uid")
	if err != nil {
		t.Fatal(err)
	}
	edits := edit.Changes["file:///test.volt"]
	if len(edits) != 2 {
		t.Fatalf("expected 2 edits, got %d: %v", len(edits), edits)
	}
}

func TestRenameRejectsInvalidName(t *testing.T) {
	d := doc(t, sample)
	pos := posOf(t, sample, "Table users", 0)
	pos.Character += protocol.UInteger(len("Table "))
	if _, err := d.Rename(pos, "not valid!"); err == nil {
		t.Fatal("expected error for invalid identifier")
	}
	if _, err := d.Rename(pos, "1234"); err == nil {
		t.Fatal("expected error for all-digit name")
	}
}

func TestHoverTable(t *testing.T) {
	d := doc(t, sample)
	pos := posOf(t, sample, "> users.id", 0)
	pos.Character += 2
	h := d.Hover(pos)
	if h == nil {
		t.Fatal("no hover for table reference")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	for _, want := range []string{"Table users", "name string", "people", "from ~base"} {
		if !strings.Contains(md, want) {
			t.Errorf("hover missing %q:\n%s", want, md)
		}
	}
}

func TestHoverDeclaredType(t *testing.T) {
	d := doc(t, sample)
	pos := posOf(t, sample, "status Status", 0)
	pos.Character += protocol.UInteger(len("status "))
	h := d.Hover(pos)
	if h == nil {
		t.Fatal("no hover for declared type reference")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	if !strings.Contains(md, "Type Status string") {
		t.Errorf("unexpected declared type hover:\n%s", md)
	}
}

func TestDocumentSymbols(t *testing.T) {
	d := doc(t, sample)
	syms := d.DocumentSymbols()
	var names []string
	for _, s := range syms {
		names = append(names, s.Name)
	}
	joined := strings.Join(names, " ")
	for _, want := range []string{"Status", "base", "users", "orders", "g"} {
		if !strings.Contains(joined, want) {
			t.Errorf("outline missing %q: %v", want, names)
		}
	}
	for _, s := range syms {
		if s.Name == "users" && len(s.Children) < 3 {
			t.Errorf("users should have >= 3 children, got %d", len(s.Children))
		}
	}
}

func TestCompletionTopLevel(t *testing.T) {
	text := sample + "\nT"
	d := doc(t, text)
	items := d.Complete(protocol.Position{Line: protocol.UInteger(strings.Count(text, "\n")), Character: 1})
	if !hasLabel(items, "Table") || !hasLabel(items, "TablePartial") || !hasLabel(items, "Type") {
		t.Fatalf("top-level completion missing element keywords: %v", labels(items))
	}
}

func TestCompletionColumnSettings(t *testing.T) {
	text := "Table t {\n  id int [p\n}\n"
	d := doc(t, text)
	items := d.Complete(protocol.Position{Line: 1, Character: protocol.UInteger(len("  id int [p"))})
	if !hasLabel(items, "pk") || !hasLabel(items, "null") {
		t.Fatalf("column settings completion missing: %v", labels(items))
	}
}

func TestCompletionDefaultValues(t *testing.T) {
	text := "Table t {\n  s bool [unique, default: \n}\n"
	d := doc(t, text)
	line := "  s bool [unique, default: "
	items := d.Complete(protocol.Position{Line: 1, Character: protocol.UInteger(len(line))})
	if !hasLabel(items, "true") || !hasLabel(items, "false") {
		t.Fatalf("default value completion missing: %v", labels(items))
	}
	if hasLabel(items, "pk") || hasLabel(items, "null") {
		t.Fatalf("column settings, or null, leaked into a default value position: %v", labels(items))
	}
}

func TestCompletionRefEndpointColumns(t *testing.T) {
	text := sample + "\nRef: x.y > users."
	d := doc(t, text)
	line := protocol.UInteger(strings.Count(text, "\n"))
	items := d.Complete(protocol.Position{Line: line, Character: protocol.UInteger(len("Ref: x.y > users."))})
	if !hasLabel(items, "name") || !hasLabel(items, "id") {
		t.Fatalf("ref endpoint completion missing columns: %v", labels(items))
	}
}

func TestCompletionInlineRefTables(t *testing.T) {
	text := "Table a {\n  x int [ref: \n}\n" + sample
	d := doc(t, text)
	items := d.Complete(protocol.Position{Line: 1, Character: protocol.UInteger(len("  x int [ref: "))})
	if !hasLabel(items, "users") || !hasLabel(items, "orders") {
		t.Fatalf("inline ref completion missing tables: %v", labels(items))
	}
}

func TestCompletionTopLevelOffersQueryElements(t *testing.T) {
	text := sample + "\nS"
	d := doc(t, text)
	items := d.Complete(protocol.Position{Line: protocol.UInteger(strings.Count(text, "\n")), Character: 1})
	for _, want := range []string{"Group", "Pred", "Select"} {
		if !hasLabel(items, want) {
			t.Errorf("top-level completion misses %s (§groups, §predicates, §selects-over-groups): %v", want, labels(items))
		}
	}
}

func TestCompletionSettingsPerHeader(t *testing.T) {
	text := sample + "\nTablePartial tp [\nType Slug string [\nTable t2 [\nSelect s for users [\nTable t3 {\n  tablename string [\n  refcount int [\n}\n"
	d := doc(t, text)
	at := func(needle string) []string {
		pos := posOf(t, text, needle, 0)
		pos.Character += protocol.UInteger(len(needle))
		return labels(d.Complete(pos))
	}
	if got := at("TablePartial tp ["); len(got) != 0 {
		t.Errorf("a TablePartial takes no settings (§tablepartial), completion offers %v", got)
	}
	if got := at("Type Slug string ["); len(got) != 0 {
		t.Errorf("a Type takes no settings (§types), completion offers %v", got)
	}
	if got := strings.Join(at("Table t2 ["), " "); got != "note:  model: " {
		t.Errorf("table settings are note and model (§table), completion offers %q", got)
	}
	if got := strings.Join(at("Select s for users ["), " "); got != "order: (" {
		t.Errorf("a select takes order: (§selects-over-groups), completion offers %q", got)
	}
	for _, needle := range []string{"  tablename string [", "  refcount int ["} {
		if got := at(needle); !slices.Contains(got, "pk") || slices.Contains(got, "model: ") || slices.Contains(got, "delete: ") {
			t.Errorf("%q is a column whose name starts like an element keyword; completion offers %v, want the column settings", needle, got)
		}
	}
}

func TestCompletionGroupBody(t *testing.T) {
	text := sample + "\nGroup gg {\n  \n}\n"
	d := doc(t, text)
	items := d.Complete(protocol.Position{Line: protocol.UInteger(strings.Count(sample, "\n") + 2), Character: 2})
	if !hasLabel(items, "users") || !hasLabel(items, "orders") || hasLabel(items, "Table") {
		t.Fatalf("a Group body completes members, tables, not element keywords (§groups): %v", labels(items))
	}
}

func TestCompletionInlineRefColumns(t *testing.T) {
	text := sample + "\nTable t4 {\n  uid int [ref: users.\n  vid int [pk, ref: users.\n  wid int [ref: > \n}\n"
	d := doc(t, text)
	for _, needle := range []string{"  uid int [ref: users.", "  vid int [pk, ref: users."} {
		pos := posOf(t, text, needle, 0)
		pos.Character += protocol.UInteger(len(needle))
		if items := d.Complete(pos); !hasLabel(items, "name") || hasLabel(items, "pk") {
			t.Errorf("%q completes the columns of users (§relationships-ref), got %v", needle, labels(items))
		}
	}
	pos := posOf(t, text, "  wid int [ref: > ", 0)
	pos.Character += protocol.UInteger(len("  wid int [ref: > "))
	if items := d.Complete(pos); hasLabel(items, "users") {
		t.Errorf("an inline ref takes no operator (§relationships-ref), completion offers tables after it: %v", labels(items))
	}
}

func TestCompletionDefaultValuesFollowTheColumnType(t *testing.T) {
	text := "Type Flag bool\nType Slug string\n\nTable t {\n  a string [default: \n  b Slug [default: \n  c Flag [default: \n}\n"
	d := doc(t, text)
	for needle, want := range map[string]bool{"  a string [default: ": false, "  b Slug [default: ": false, "  c Flag [default: ": true} {
		pos := posOf(t, text, needle, 0)
		pos.Character += protocol.UInteger(len(needle))
		if got := hasLabel(d.Complete(pos), "true"); got != want {
			t.Errorf("%q offers true: %v, want %v (§default-values)", needle, got, want)
		}
	}
}

func TestHoverShowsAStringDefaultQuoted(t *testing.T) {
	d := doc(t, sample)
	h := d.Hover(posOf(t, sample, "status Status", 0))
	if h == nil {
		t.Fatal("no hover on the status column")
	}
	if md := h.Contents.(protocol.MarkupContent).Value; !strings.Contains(md, "default: 'active'") {
		t.Errorf("a string default hovers as written, quoted (§default-values): %s", md)
	}
}

func TestValueStringSpellsACompositeEndpoint(t *testing.T) {
	f, _ := parser.ParseFile("x.volt", "Table periods {\n  a int [ref: merchants.(id, country_code)]\n  b int [ref: merchants.id]\n}\n")
	var got []string
	for _, item := range f.Decls[0].(*ast.Table).Body {
		if col, ok := item.(*ast.Column); ok {
			got = append(got, valueString(col.Settings.Get("ref").Value))
		}
	}
	if want := []string{"merchants.(id, country_code)", "merchants.id"}; !slices.Equal(got, want) {
		t.Errorf("ref endpoints render as %q, want %q (§relationships-ref)", got, want)
	}
}

func TestCompletionPartialInjection(t *testing.T) {
	text := sample + "\nTable extra {\n  ~\n}\n"
	d := doc(t, text)
	line := protocol.UInteger(strings.Count(sample, "\n") + 2)
	items := d.Complete(protocol.Position{Line: line, Character: 3})
	if !hasLabel(items, "base") {
		t.Fatalf("~ completion missing partial: %v", labels(items))
	}
}

func TestCompletionTypesIncludeBuiltins(t *testing.T) {
	text := sample + "\nTable extra {\n  col v\n}\n"
	d := doc(t, text)
	line := protocol.UInteger(strings.Count(sample, "\n") + 2)
	items := d.Complete(protocol.Position{Line: line, Character: protocol.UInteger(len("  col v"))})
	if !hasLabel(items, "string") || !hasLabel(items, "time.Time") || !hasLabel(items, "Status") {
		t.Fatalf("type completion missing: %v", labels(items))
	}
}

func TestCompletionTypesIncludeDeclaredTypes(t *testing.T) {
	text := "Type money.Money int from 'example.com/app/money'\nType Slug string\n\nTable products {\n  col S\n}\n"
	d := doc(t, text)
	items := d.Complete(protocol.Position{Line: 4, Character: protocol.UInteger(len("  col S"))})
	for _, want := range []string{"money.Money", "Slug"} {
		if !hasLabel(items, want) {
			t.Errorf("type completion missing declared type %q: %v", want, labels(items))
		}
	}
	for _, it := range items {
		if (it.Label == "money.Money" || it.Label == "Slug") && (it.Detail == nil || *it.Detail != "Type") {
			t.Errorf("declared type %q should carry the detail Type, got %v", it.Label, it.Detail)
		}
	}
}

func TestDocumentSymbolsTypeDecl(t *testing.T) {
	d := doc(t, "Type money.Money int from 'example.com/app/money'\nType Slug string\n")
	syms := d.DocumentSymbols()
	got := map[string]protocol.DocumentSymbol{}
	for _, s := range syms {
		got[s.Name] = s
	}
	for _, want := range []string{"money.Money", "Slug"} {
		s, ok := got[want]
		if !ok {
			t.Errorf("outline missing Type %q: %v", want, syms)
			continue
		}
		if s.Detail == nil || *s.Detail != "Type" {
			t.Errorf("Type %q detail = %v, want Type", want, s.Detail)
		}
	}
}

func TestUTF16Conversion(t *testing.T) {
	text := "Table \"𝔘ser\" {\n  id int [pk]\n}\n"
	d := doc(t, text)
	if len(d.Diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", d.Diags)
	}
	off := d.FromLSP(protocol.Position{Line: 1, Character: 2})
	if d.Text[off:off+2] != "id" {
		t.Errorf("FromLSP landed on %q", d.Text[off:off+2])
	}
	lsp := d.ToLSP(tokenPosition(t, d, `"𝔘ser"`))
	if lsp.Line != 0 || lsp.Character != protocol.UInteger(len(`Table `)) {
		t.Errorf("ToLSP = %+v", lsp)
	}
}

func tokenPosition(t *testing.T, d *Document, needle string) (p token.Position) {
	t.Helper()
	off := strings.Index(d.Text, needle)
	if off < 0 {
		t.Fatalf("needle %q not found", needle)
	}
	line := strings.Count(d.Text[:off], "\n") + 1
	lineStart := strings.LastIndex(d.Text[:off], "\n") + 1
	return token.At("", off, line, len([]rune(d.Text[lineStart:off]))+1)
}

func hasLabel(items []protocol.CompletionItem, label string) bool {
	for _, it := range items {
		if it.Label == label {
			return true
		}
	}
	return false
}

func labels(items []protocol.CompletionItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Label
	}
	return out
}

func TestCRLFDocumentPositions(t *testing.T) {
	text := "Table posts {\r\n  id int [pk]\r\n  title text [null]\r\n  title text [null]\r\n}\r\n\r\nRef: posts.id > posts.title\r\n"
	d := doc(t, text)
	found := false
	for _, dg := range d.LSPDiagnostics() {
		if !strings.Contains(dg.Message, "duplicate column") {
			continue
		}
		found = true
		if r := dg.Range; r.Start.Line != 3 || r.Start.Character != 2 || r.End.Line != 3 || r.End.Character != 7 {
			t.Errorf("duplicate column underlined at %+v, want line 3, characters 2 to 7", r)
		}
	}
	if !found {
		t.Fatalf("no duplicate column diagnostic: %v", d.Diags)
	}
	loc := d.Definition(posOf(t, text, "posts", 1))
	if loc == nil {
		t.Fatal("no definition for the table named in the Ref")
	}
	if s := loc.Range.Start; s.Line != 0 || s.Character != 6 {
		t.Errorf("definition at %+v, want line 0 character 6", loc.Range)
	}
}
