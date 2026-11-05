package lsp

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/parser"
	"github.com/Piechutowski/volt/lang/token"
)

const sample = `type Status string

table_partial base {
  id int [pk]
}

// people
table users {
  ~base
  name string [unique]
  status Status [null, default: "active"]
}

table orders {
  id int [pk]
  user_id int [null]
  owner_name string [null, ref: users.name]
}

ref: orders.user_id > users.id

group g {
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
	d := doc(t, "table t {\n  id int [pk\n}\n")
	got := d.LSPDiagnostics()
	if len(got) == 0 {
		t.Fatal("expected diagnostics for unclosed settings")
	}
	if *got[0].Severity != protocol.DiagnosticSeverityError {
		t.Errorf("expected error severity, got %v", *got[0].Severity)
	}
}

func TestVetWarningsSurface(t *testing.T) {
	d := doc(t, "table t {\n  id int [pk]\n  indexes {\n  }\n}\n")
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
	d := doc(t, "table status {\n  id int [pk]\n}\ntable other {\n  id int [pk]\n  s int [null, ref: status.id]\n}\n")
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
	want := posOf(t, sample, "table users {", 0)
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
	declPos := posOf(t, sample, "table users {", 0)
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
	want := posOf(t, sample, "table_partial base", 0)
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
		t.Errorf("type definition line = %d, want 0", loc.Range.Start.Line)
	}
}

func TestReferencesForTable(t *testing.T) {
	d := doc(t, sample)
	pos := posOf(t, sample, "table users {", 0)
	pos.Character += protocol.UInteger(len("table "))
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
	pos := posOf(t, sample, "table users", 0)
	pos.Character += protocol.UInteger(len("table "))
	if _, err := d.Rename(pos, "not valid!"); err == nil {
		t.Fatal("expected error for invalid identifier")
	}
	if _, err := d.Rename(pos, "1234"); err == nil {
		t.Fatal("expected error for all-digit name")
	}
	for _, name := range []string{"2fa_codes", "\u0301x", "x\u2014y", "x\u00b2", "a\u00a0b"} {
		if _, err := d.Rename(pos, name); err == nil {
			t.Errorf("Rename to %q accepted, a name the scanner refuses (§identifiers)", name)
		}
	}
	for _, name := range []string{"ok_name", "_", "\u017c\u00f3\u0142w", "x\u0663"} {
		if _, err := d.Rename(pos, name); err != nil {
			t.Errorf("Rename to %q refused: %v", name, err)
		}
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
	for _, want := range []string{"table users", "name string", "people", "from ~base"} {
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
	if !strings.Contains(md, "type Status string") {
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
	if !hasLabel(items, "table") || !hasLabel(items, "table_partial") || !hasLabel(items, "type") {
		t.Fatalf("top-level completion missing element keywords: %v", labels(items))
	}
}

func TestCompletionColumnSettings(t *testing.T) {
	text := "table t {\n  id int [p\n}\n"
	d := doc(t, text)
	items := d.Complete(protocol.Position{Line: 1, Character: protocol.UInteger(len("  id int [p"))})
	if !hasLabel(items, "pk") || !hasLabel(items, "null") || !hasLabel(items, "unique") || !hasLabel(items, "index") {
		t.Fatalf("column settings completion missing pk, null, unique or index (§columns, §indexes): %v", labels(items))
	}
}

func TestCompletionDefaultValues(t *testing.T) {
	text := "table t {\n  s bool [unique, default: \n}\n"
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
	text := sample + "\nref: x.y > users."
	d := doc(t, text)
	line := protocol.UInteger(strings.Count(text, "\n"))
	items := d.Complete(protocol.Position{Line: line, Character: protocol.UInteger(len("ref: x.y > users."))})
	if !hasLabel(items, "name") || !hasLabel(items, "id") {
		t.Fatalf("ref endpoint completion missing columns: %v", labels(items))
	}
}

func TestCompletionInlineRefTables(t *testing.T) {
	text := "table a {\n  x int [ref: \n}\n" + sample
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
	for _, want := range []string{"group", "pred", "select"} {
		if !hasLabel(items, want) {
			t.Errorf("top-level completion misses %s (§groups, §predicates, §selects-over-groups): %v", want, labels(items))
		}
	}
}

func TestCompletionSettingsPerHeader(t *testing.T) {
	text := sample + "\ntable_partial tp [\ntype Slug string [\ntable t2 [\nselect s for users [\ntable t3 {\n  tablename string [\n  refcount int [\n}\n"
	d := doc(t, text)
	at := func(needle string) []string {
		pos := posOf(t, text, needle, 0)
		pos.Character += protocol.UInteger(len(needle))
		return labels(d.Complete(pos))
	}
	if got := at("table_partial tp ["); len(got) != 0 {
		t.Errorf("a TablePartial takes no settings (§tablepartial), completion offers %v", got)
	}
	if got := at("type Slug string ["); len(got) != 0 {
		t.Errorf("a Type takes no settings (§types), completion offers %v", got)
	}
	if got := strings.Join(at("table t2 ["), " "); got != "model: " {
		t.Errorf("model is the one table setting (§table), completion offers %q", got)
	}
	if got := strings.Join(at("select s for users ["), " "); got != "order: ( group: ( limit: " {
		t.Errorf("a select takes order:, group: and limit: (§selects-over-groups, §settings-whitelists), completion offers %q", got)
	}
	for _, needle := range []string{"  tablename string [", "  refcount int ["} {
		if got := at(needle); !slices.Contains(got, "pk") || slices.Contains(got, "model: ") || slices.Contains(got, "delete: ") {
			t.Errorf("%q is a column whose name starts like an element keyword; completion offers %v, want the column settings", needle, got)
		}
	}
}

func TestCompletionGroupBody(t *testing.T) {
	text := sample + "\ngroup gg {\n  \n}\n"
	d := doc(t, text)
	items := d.Complete(protocol.Position{Line: protocol.UInteger(strings.Count(sample, "\n") + 2), Character: 2})
	if !hasLabel(items, "users") || !hasLabel(items, "orders") || hasLabel(items, "table") {
		t.Fatalf("a Group body completes members, tables, not element keywords (§groups): %v", labels(items))
	}
}

func TestCompletionInlineRefColumns(t *testing.T) {
	text := sample + "\ntable t4 {\n  uid int [ref: users.\n  vid int [pk, ref: users.\n  wid int [ref: > \n}\n"
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
	text := "type Flag bool\ntype Slug string\n\ntable t {\n  a string [default: \n  b Slug [default: \n  c Flag [default: \n}\n"
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
	if md := h.Contents.(protocol.MarkupContent).Value; !strings.Contains(md, `default: "active"`) {
		t.Errorf("a string default hovers as written, quoted (§default-values): %s", md)
	}
}

func TestValueStringSpellsACompositeEndpoint(t *testing.T) {
	f, _ := parser.ParseFile("x.volt", "table periods {\n  a int [ref: merchants.(id, country_code)]\n  b int [ref: merchants.id]\n}\n")
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
	text := sample + "\ntable extra {\n  ~\n}\n"
	d := doc(t, text)
	line := protocol.UInteger(strings.Count(sample, "\n") + 2)
	items := d.Complete(protocol.Position{Line: line, Character: 3})
	if !hasLabel(items, "base") {
		t.Fatalf("~ completion missing partial: %v", labels(items))
	}
}

func TestCompletionTypesIncludeBuiltins(t *testing.T) {
	text := sample + "\ntable extra {\n  col v\n}\n"
	d := doc(t, text)
	line := protocol.UInteger(strings.Count(sample, "\n") + 2)
	items := d.Complete(protocol.Position{Line: line, Character: protocol.UInteger(len("  col v"))})
	if !hasLabel(items, "string") || !hasLabel(items, "time.Time") || !hasLabel(items, "Status") {
		t.Fatalf("type completion missing: %v", labels(items))
	}
}

func TestCompletionTypesIncludeDeclaredTypes(t *testing.T) {
	text := "type money.Money int from \"example.com/app/money\"\ntype Slug string\n\ntable products {\n  col S\n}\n"
	d := doc(t, text)
	items := d.Complete(protocol.Position{Line: 4, Character: protocol.UInteger(len("  col S"))})
	for _, want := range []string{"money.Money", "Slug"} {
		if !hasLabel(items, want) {
			t.Errorf("type completion missing declared type %q: %v", want, labels(items))
		}
	}
	for _, it := range items {
		if (it.Label == "money.Money" || it.Label == "Slug") && (it.Detail == nil || *it.Detail != "type") {
			t.Errorf("declared type %q should carry the detail Type, got %v", it.Label, it.Detail)
		}
	}
}

func TestDocumentSymbolsTypeDecl(t *testing.T) {
	d := doc(t, "type money.Money int from \"example.com/app/money\"\ntype Slug string\n")
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
		if s.Detail == nil || *s.Detail != "type" {
			t.Errorf("type %q detail = %v, want Type", want, s.Detail)
		}
	}
}

func TestUTF16Conversion(t *testing.T) {
	text := "table 𝔘ser {\n  id int [pk]\n}\n"
	d := doc(t, text)
	if len(d.Diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", d.Diags)
	}
	off := d.FromLSP(protocol.Position{Line: 1, Character: 2})
	if d.Text[off:off+2] != "id" {
		t.Errorf("FromLSP landed on %q", d.Text[off:off+2])
	}
	lsp := d.ToLSP(tokenPosition(t, d, `𝔘ser`))
	if lsp.Line != 0 || lsp.Character != protocol.UInteger(len(`table `)) {
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
	text := "table posts {\r\n  id int [pk]\r\n  title text [null]\r\n  title text [null]\r\n}\r\n\r\nref: posts.id > posts.title\r\n"
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

func TestCompletionOffersTheOneSpelling(t *testing.T) {
	trimmed := func(words []string) []string {
		out := make([]string, len(words))
		for i, w := range words {
			out[i] = strings.TrimRight(w, " ({:")
		}
		return out
	}
	elements := trimmed(elementKeywords)
	for _, w := range parser.ElementWords {
		if !slices.Contains(elements, w) {
			t.Errorf("top-level completion lacks %s (§keywords)", w)
		}
	}
	scope := trimmed(scopeBodyKeywords)
	for _, v := range parser.Verbs {
		if !slices.Contains(scope, v) {
			t.Errorf("scope completion lacks the verb %s (§keywords)", v)
		}
	}
	for _, w := range scope {
		if !slices.Contains(parser.Verbs, w) && w != "resources" && w != "dataset" && w != "scope" {
			t.Errorf("scope completion offers %q, which opens no scope item (§keywords, §routes)", w)
		}
	}
	for ctx, words := range settingsByContext {
		for _, w := range trimmed(words) {
			if !slices.Contains(parser.SettingNames, w) {
				t.Errorf("%s settings complete %q, which is no setting name (§keywords)", ctx, w)
			}
		}
	}
	if !slices.Equal(refActionValues, parser.RefActions) {
		t.Errorf("referential actions complete as %v, want %v (§keywords)", refActionValues, parser.RefActions)
	}
	d := doc(t, "table t {\n  id int [pk]\n  \n}\n")
	items := labels(d.Complete(protocol.Position{Line: 2, Character: 2}))
	for _, w := range []string{"indexes", "validators", "records"} {
		if !slices.Contains(items, w) {
			t.Errorf("table body completion %v lacks %s", items, w)
		}
	}
	for _, w := range []string{"Indexes", "Validators", "Records"} {
		if slices.Contains(items, w) {
			t.Errorf("table body completion %v offers %s, a spelling Volt refuses", items, w)
		}
	}
}

// TestSettingsCompleteTheWhitelist holds setting completion to the spec
// (§settings-whitelists, §relationships-ref, editor.md section 5): every
// construct of the whitelist table, and a ref element, completes exactly
// the settings the spec lists for it.
func TestSettingsCompleteTheWhitelist(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "docs", "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	spec := string(b)
	start := strings.Index(spec, "\n## Settings whitelists\n")
	end := strings.Index(spec[start+1:], "\n## ")
	if start < 0 || end < 0 {
		t.Fatal("no §settings-whitelists section in the spec")
	}
	contexts := map[string]string{"`scope`": "scope", "route": "route", "resources": "resources", "dataset": "dataset", "Select": "select", "`migration`": "migration"}
	want := map[string][]string{"ref": {"as", "delete", "reverse", "update"}}
	for _, line := range strings.Split(spec[start:start+1+end], "\n") {
		cells := strings.Split(line, "|")
		if len(cells) != 4 {
			continue
		}
		ctx, ok := contexts[strings.TrimSpace(cells[1])]
		if !ok {
			continue
		}
		for _, name := range strings.Split(cells[2], ",") {
			want[ctx] = append(want[ctx], strings.Trim(strings.TrimSpace(name), "`"))
		}
	}
	if len(want) != len(contexts)+1 {
		t.Fatalf("read the whitelist of %d constructs from the spec, want %d", len(want)-1, len(contexts))
	}
	for ctx, names := range want {
		var got []string
		for _, w := range settingsByContext[ctx] {
			got = append(got, strings.TrimRight(w, " ({:"))
		}
		slices.Sort(got)
		slices.Sort(names)
		if !slices.Equal(got, names) {
			t.Errorf("%s settings complete %v; the spec lists %v", ctx, got, names)
		}
	}
}

func TestHoverDocComment(t *testing.T) {
	src := "table posts {\n  id int [pk]\n  // the column\n  author_id int\n\n  indexes {\n    // Lists a writer's posts.\n    (author_id, id)\n  }\n}\n"
	d := doc(t, src)
	h := d.Hover(posOf(t, src, "(author_id, id)", 0))
	if h == nil || h.Contents.(protocol.MarkupContent).Value != "Lists a writer's posts." {
		t.Errorf("hover on a documented index = %+v, want its doc text (§doc-comments)", h)
	}
	h = d.Hover(posOf(t, src, "author_id int", 0))
	if h == nil || !strings.Contains(h.Contents.(protocol.MarkupContent).Value, "the column") {
		t.Errorf("hover on a documented column = %+v, want its doc text after the column (§doc-comments)", h)
	}
	if h := d.Hover(posOf(t, src, "id int", 0)); h != nil && strings.Contains(h.Contents.(protocol.MarkupContent).Value, "---") {
		t.Errorf("hover on an undocumented column carries a doc: %+v", h)
	}
}

func TestHoverDocCommentInRoutes(t *testing.T) {
	src := "package app\n\n// The browser stack.\npipeline browser {\n\t// reads the session\n\tuse Session\n}\n\nscope / {\n\t// The front page.\n\tget / Home.Index {\n\t\t// the posts it lists\n\t\tposts db.PostList()\n\t}\n\tscope /admin {\n\t\t// The admin home.\n\t\tget / Admin.Home\n\t}\n}\n"
	d := doc(t, src)
	for needle, want := range map[string]string{
		"use Session":      "reads the session",
		"get / Home.Index": "The front page.",
		"posts db.":        "the posts it lists",
		"get / Admin.Home": "The admin home.",
	} {
		h := d.Hover(posOf(t, src, needle, 0))
		if h == nil || !strings.Contains(h.Contents.(protocol.MarkupContent).Value, want) {
			t.Errorf("hover at %q = %+v, want the doc text %q (§doc-comments)", needle, h, want)
		}
	}
}
