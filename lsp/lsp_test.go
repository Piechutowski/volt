package lsp

import (
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/lang/token"
)

const sample = `Enum status {
  active [note: 'ok']
  retired
}

TablePartial base {
  id int [pk]
}

Table users as U [note: 'people'] {
  ~base
  name string [not null, unique]
  status status [default: status.active]
}

Table core.orders {
  id int [pk]
  user_id int
  owner_name string [ref: > U.name]
}

Ref: users.id < core.orders.user_id

TableGroup g {
  users
  core.orders
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
	d := doc(t, "Table t {\n  id int [pk, null]\n}\n")
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
	d := doc(t, "Table status {\n  id int [pk]\n}\nTable other {\n  id int [pk]\n  s int [ref: > status.id]\n}\n")
	for _, dg := range d.Diags {
		if strings.Contains(dg.Code, "modelname") {
			t.Fatalf("modelname analyzer should be excluded, got %v", dg)
		}
	}
}

func TestDefinitionFromRefEndpoint(t *testing.T) {
	d := doc(t, sample)
	pos := posOf(t, sample, "users.id <", 0)
	loc := d.Definition(pos)
	if loc == nil {
		t.Fatal("no definition for ref endpoint table")
	}
	want := posOf(t, sample, "Table users as U", 0)
	if loc.Range.Start.Line != want.Line+0 {
		t.Errorf("definition line = %d, want %d", loc.Range.Start.Line, want.Line)
	}
}

func TestDefinitionAliasResolvesToTable(t *testing.T) {
	d := doc(t, sample)
	pos := posOf(t, sample, "U.name", 0)
	loc := d.Definition(pos)
	if loc == nil {
		t.Fatal("no definition via alias")
	}
	declPos := posOf(t, sample, "users", 0)
	if loc.Range.Start.Line != declPos.Line {
		t.Errorf("alias definition line = %d, want %d", loc.Range.Start.Line, declPos.Line)
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

func TestDefinitionEnumType(t *testing.T) {
	d := doc(t, sample)
	pos := posOf(t, sample, "status status", 0)
	pos.Character += protocol.UInteger(len("status "))
	loc := d.Definition(pos)
	if loc == nil {
		t.Fatal("no definition for enum type reference")
	}
	if loc.Range.Start.Line != 0 {
		t.Errorf("enum definition line = %d, want 0", loc.Range.Start.Line)
	}
}

func TestReferencesForTable(t *testing.T) {
	d := doc(t, sample)
	pos := posOf(t, sample, "Table users as U", 0)
	pos.Character += protocol.UInteger(len("Table "))
	refs := d.References(pos, true)
	if len(refs) < 4 {
		t.Fatalf("expected >= 4 references for users, got %d: %v", len(refs), refs)
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
	pos := posOf(t, sample, "users.id <", 0)
	h := d.Hover(pos)
	if h == nil {
		t.Fatal("no hover for table reference")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	for _, want := range []string{"Table users as U", "name string", "people", "from ~base"} {
		if !strings.Contains(md, want) {
			t.Errorf("hover missing %q:\n%s", want, md)
		}
	}
}

func TestHoverEnumValue(t *testing.T) {
	d := doc(t, sample)
	pos := posOf(t, sample, "status.active", 0)
	pos.Character += protocol.UInteger(len("status."))
	h := d.Hover(pos)
	if h == nil {
		t.Fatal("no hover for enum value")
	}
	md := h.Contents.(protocol.MarkupContent).Value
	if !strings.Contains(md, "status.active") || !strings.Contains(md, "ok") {
		t.Errorf("unexpected enum value hover:\n%s", md)
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
	for _, want := range []string{"status", "base", "users", "core.orders", "g"} {
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
	if !hasLabel(items, "pk") || !hasLabel(items, "not null") {
		t.Fatalf("column settings completion missing: %v", labels(items))
	}
}

func TestCompletionDefaultEnumValues(t *testing.T) {
	text := "Enum status {\n  active\n  banned\n}\n\nTable t {\n  s status [not null, default: status.\n}\n"
	d := doc(t, text)
	line := "  s status [not null, default: status."
	items := d.Complete(protocol.Position{Line: 6, Character: protocol.UInteger(len(line))})
	if !hasLabel(items, "active") || !hasLabel(items, "banned") {
		t.Fatalf("enum value completion missing: %v", labels(items))
	}
	if hasLabel(items, "pk") || hasLabel(items, "not null") {
		t.Fatalf("column settings leaked into an enum value position: %v", labels(items))
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
	text := "Table a {\n  x int [ref: > \n}\n" + sample
	d := doc(t, text)
	items := d.Complete(protocol.Position{Line: 1, Character: protocol.UInteger(len("  x int [ref: > "))})
	if !hasLabel(items, "users") || !hasLabel(items, "core.orders") {
		t.Fatalf("inline ref completion missing tables: %v", labels(items))
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

func TestCompletionTypesIncludeEnums(t *testing.T) {
	text := sample + "\nTable extra {\n  col v\n}\n"
	d := doc(t, text)
	line := protocol.UInteger(strings.Count(sample, "\n") + 2)
	items := d.Complete(protocol.Position{Line: line, Character: protocol.UInteger(len("  col v"))})
	if !hasLabel(items, "string") || !hasLabel(items, "status") {
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
	text := "Table posts {\r\n  id int [pk]\r\n  title text\r\n  title text\r\n}\r\n\r\nRef: posts.id > posts.title\r\n"
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
