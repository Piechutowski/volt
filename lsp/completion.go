package lsp

import (
	"regexp"
	"strings"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/parser"
	"github.com/Piechutowski/volt/lsp/protocol"
)

var (
	elementKeywords = []string{
		"table", "table_partial", "type", "ref",
		"records", "migration",
		"package ", "import", "pipeline", "scope",
		"group", "pred", "select",
	}
	tableBodyKeywords     = []string{"indexes", "validators", "records"}
	migrationBodyKeywords = []string{
		"table_create ", "index_create ", "table_drop ", "index_drop ",
		"table_rename ", "table_alter ", "table_rebuild ", "exec ", "up {", "down {",
	}
	alterBodyKeywords = []string{"add ", "drop ", "rename "}
	scopeBodyKeywords = []string{
		"get ", "post ", "patch ", "delete ", "options ", "head ",
		"any ", "resources ", "dataset ", "scope ",
	}

	settingsByContext = map[string][]string{
		"column":    {"pk", "null", "unique", "index", "autoincrement", "required", "default: ", "ref: ", "tag: "},
		"table":     {"model: "},
		"select":    {"order: (", "group: (", "limit: "},
		"index":     {"name: ", "unique", "where: "},
		"migration": {"irreversible", "no_transaction"},
		"ref":       {"delete: ", "update: ", "as: ", "reverse: "},
		"scope":     {"pipe: ", "error_handler: ", "name: "},
		"route":     {"name: ", "timeout: ", "body: ", "only: ("},
		"resources": {"only: (", "except: (", "param: ", "singular: ", "api", "default"},
		"dataset":   {"strip: ", "only: (", "except: ("},

		"migrationcolumn": {"pk", "null", "unique", "index", "autoincrement", "default: ", "ref: ", "delete: ", "update: "},
		"rebuildcolumn":   {"pk", "null", "unique", "index", "autoincrement", "default: ", "ref: ", "delete: ", "update: ", "from: "},
		"migrationindex":  {"unique", "name: ", "where: "},
		"standaloneindex": {"unique", "name: ", "where: "},
	}

	builtinTypes = check.Builtins

	refActionValues = parser.RefActions

	voltVerbSet = func() map[string]bool {
		m := map[string]bool{}
		for _, v := range parser.Verbs {
			m[v] = true
		}
		return m
	}()

	partialInjectRE = regexp.MustCompile(`~\s*` + identChar + `*$`)
	dotChainRE      = regexp.MustCompile(`(` + identChar + `+)(\.(` + identChar + `+))?\.$`)
	refValueRE      = regexp.MustCompile(identBefore + `ref\s*:\s*$`)
	refDotRE        = regexp.MustCompile(identBefore + `ref\s*:\s*(` + identChar + `+)\.$`)
	refHeadRE       = regexp.MustCompile(`^\s*ref\s*:\s*` + identChar + `*$`)
	refTargetRE     = regexp.MustCompile(`^\s*ref\s*:[^>]*>\s*` + identChar + `*$`)
	defaultValueRE  = regexp.MustCompile(identBefore + `default\s*:\s*$`)
	columnTypeRE    = regexp.MustCompile(`^\s*(?:add\s+)?` + identChar + `+\s+([` + identCharSet + `.\[\]]+)\s*\[`)
	actionValueRE   = regexp.MustCompile(identBefore + `(delete|update)\s*:\s*` + identChar + `*$`)
	columnShapeRE   = regexp.MustCompile(`^\s*(` + identChar + `+)\s+[` + identCharSet + `.]*$`)
	lineStartRE     = regexp.MustCompile(`^\s*[` + identCharSet + `~]*$`)
	shapeHeadRE     = regexp.MustCompile(`^table_(create|rebuild|drop)` + identAfter)
	upDownHeadRE    = regexp.MustCompile(`^(up|down)\s*\{`)
)

// Complete computes completion items for the position.
func (d *Document) Complete(pos protocol.Position) []protocol.CompletionItem {
	offset := d.FromLSP(pos)
	lineStart := d.lineOffsets[min(int(pos.Line), len(d.lineOffsets)-1)]
	prefix := d.Text[lineStart:offset]
	ctx, outer := d.blockContexts(int(pos.Line))

	if items := d.voltComplete(prefix, ctx == "pred"); items != nil {
		return items
	}

	if (ctx == "table" || ctx == "tablepartial") && partialInjectRE.MatchString(prefix) {
		return d.partialItems()
	}

	inSettings := strings.Count(prefix, "[") > strings.Count(prefix, "]")
	if inSettings {
		top := true
		if pos.Line > 0 {
			start, _ := d.blockContexts(int(pos.Line) - 1)
			top = start == ""
		}
		return d.settingsComplete(prefix, ctx, outer, top)
	}

	if m := dotChainRE.FindStringSubmatch(prefix); m != nil {
		return d.dotChainComplete(m)
	}

	if ctx == "ref" || ctx == "" && (refHeadRE.MatchString(prefix) || refTargetRE.MatchString(prefix)) {
		return d.tableItems()
	}

	switch ctx {
	case "":
		if lineStartRE.MatchString(prefix) {
			return keywordItems(elementKeywords)
		}
	case "group":
		if lineStartRE.MatchString(prefix) {
			return d.groupMemberItems(strings.TrimSpace(prefix))
		}
	case "table", "tablepartial":
		if columnShapeRE.MatchString(prefix) && !lineStartRE.MatchString(prefix) {
			return d.typeItems()
		}
		if lineStartRE.MatchString(prefix) {
			items := keywordItems(tableBodyKeywords)
			items = append(items, d.partialInjectionItems()...)
			return items
		}
	case "scope":
		if lineStartRE.MatchString(prefix) {
			return keywordItems(scopeBodyKeywords)
		}
	case "pipeline":
		if lineStartRE.MatchString(prefix) {
			return keywordItems([]string{"use "})
		}
	case "migration":
		if lineStartRE.MatchString(prefix) {
			return keywordItems(migrationBodyKeywords)
		}
	case "alter":
		if lineStartRE.MatchString(prefix) {
			return keywordItems(alterBodyKeywords)
		}
	case "shape", "rebuild":
		if columnShapeRE.MatchString(prefix) && !lineStartRE.MatchString(prefix) {
			return keywordItemsKind(builtinTypes, protocol.CompletionItemKindStruct)
		}
		if lineStartRE.MatchString(prefix) {
			return keywordItems([]string{"indexes"})
		}
	}
	return nil
}

func (d *Document) blockContexts(line int) (ctx, outer string) {
	line = min(line, len(d.lineOffsets)-1)
	depth := 0
	for l := line; l >= 0; l-- {
		text := d.lineText(l)
		if l == line {
			text = strings.SplitN(text, "//", 2)[0]
		}
		depth += strings.Count(text, "}") - strings.Count(text, "{")
		if depth >= 0 {
			continue
		}
		depth = 0
		head := headContext(strings.TrimSpace(text))
		switch {
		case head == "":
		case ctx == "":
			ctx = head
			if head != "indexes" {
				return ctx, ""
			}
		default:
			return ctx, head
		}
	}
	return ctx, ""
}

func headContext(head string) string {
	switch {
	case strings.HasPrefix(head, "indexes"):
		return "indexes"
	case strings.HasPrefix(head, "validators"):
		return "validators"
	case strings.HasPrefix(head, "table_partial"):
		return "tablepartial"
	case strings.HasPrefix(head, "group"):
		return "group"
	case strings.HasPrefix(head, "table_rebuild"):
		return "rebuild"
	case shapeHeadRE.MatchString(head):
		return "shape"
	case strings.HasPrefix(head, "table_alter"):
		return "alter"
	case strings.HasPrefix(head, "table"):
		return "table"
	case strings.HasPrefix(head, "records"):
		return "records"
	case strings.HasPrefix(head, "ref"):
		return "ref"
	case strings.HasPrefix(head, "pipeline"):
		return "pipeline"
	case strings.HasPrefix(head, "pred ") || strings.HasPrefix(head, "pred{"):
		return "pred"
	case strings.HasPrefix(head, "scope"):
		return "scope"
	case strings.HasPrefix(head, "migration"), upDownHeadRE.MatchString(head):
		return "migration"
	}
	return ""
}

func (d *Document) settingsComplete(prefix, ctx, outer string, top bool) []protocol.CompletionItem {
	segment := prefix[strings.LastIndexAny(prefix, "[,")+1:]

	if refValueRE.MatchString(segment) {
		return d.tableItems()
	}
	if m := refDotRE.FindStringSubmatch(segment); m != nil {
		if ti := d.tableNamed(m[1]); ti != nil {
			return d.columnItems(ti)
		}
		return nil
	}
	if defaultValueRE.MatchString(segment) {
		if d.columnIsBool(prefix) {
			return keywordItemsKind([]string{"true", "false"}, protocol.CompletionItemKindValue)
		}
		return nil
	}
	if actionValueRE.MatchString(segment) {
		return keywordItemsKind(refActionValues, protocol.CompletionItemKindValue)
	}

	kind := "column"
	head := strings.TrimSpace(prefix)
	verb := strings.SplitN(head, " ", 2)[0]
	header := func(word string) bool { return top && strings.HasPrefix(head, word) }
	switch {
	case header("table_partial"), header("group"), header("type "):
		return nil
	case header("table"):
		kind = "table"
	case header("ref"):
		kind = "ref"
	case header("select"):
		kind = "select"
	case strings.HasPrefix(head, "scope") && (top || ctx == "scope"):
		kind = "scope"
	case header("migration"):
		kind = "migration"
	case strings.HasPrefix(head, "resources") && ctx == "scope":
		kind = "resources"
	case strings.HasPrefix(head, "dataset") && ctx == "scope":
		kind = "dataset"
	case voltVerbSet[verb] && ctx == "scope":
		kind = "route"
	case (strings.HasPrefix(head, "index_create") || strings.HasPrefix(head, "index_drop")) && ctx == "migration":
		kind = "standaloneindex"
	default:
		switch ctx {
		case "indexes":
			kind = "index"
			if outer == "shape" || outer == "rebuild" {
				kind = "migrationindex"
			}
		case "ref":
			kind = "ref"
		case "shape", "alter":
			kind = "migrationcolumn"
		case "rebuild":
			kind = "rebuildcolumn"
		}
	}
	return keywordItemsKind(settingsByContext[kind], protocol.CompletionItemKindProperty)
}

func (d *Document) columnIsBool(prefix string) bool {
	m := columnTypeRE.FindStringSubmatch(prefix)
	if m == nil {
		return false
	}
	if m[1] == "bool" {
		return true
	}
	for _, info := range d.schemas() {
		for _, ti := range info.Types {
			if ti.Key == m[1] {
				return ti.Storage.Storage == "bool"
			}
		}
	}
	return false
}

func (d *Document) groupMemberItems(partial string) []protocol.CompletionItem {
	if d.vpkg == nil || d.vindex == nil {
		return d.tableItems()
	}
	items := d.packageTableItems(d.vpkg.Path, partial)
	return append(items, d.namedDefItems("group", partial, protocol.CompletionItemKindStruct)...)
}

func (d *Document) dotChainComplete(m []string) []protocol.CompletionItem {
	first := m[1]
	second := m[3]
	var items []protocol.CompletionItem

	if second != "" {
		return items
	}
	if ti := d.tableNamed(first); ti != nil {
		items = append(items, d.columnItems(ti)...)
	}
	return items
}

func (d *Document) schemas() []*check.Info {
	infos := []*check.Info{d.Info}
	if d.vpkg != nil && d.vpkg.Schema() != nil {
		infos = append(infos, d.vpkg.Schema())
	}
	return infos
}

func (d *Document) tableNamed(name string) *check.TableInfo {
	if ti, ok := d.Index.Tables[name]; ok {
		return ti
	}
	if d.vpkg != nil && d.vpkg.Schema() != nil {
		return d.vpkg.Schema().Table(name)
	}
	return nil
}

func (d *Document) tableItems() []protocol.CompletionItem {
	var items []protocol.CompletionItem
	seen := map[string]bool{}
	for _, info := range d.schemas() {
		for _, ti := range info.Tables {
			label := ti.Decl.Name.Name()
			if !seen[label] {
				seen[label] = true
				items = append(items, item(label, protocol.CompletionItemKindStruct, "table"))
			}
		}
	}
	return items
}

func (d *Document) columnItems(ti *check.TableInfo) []protocol.CompletionItem {
	var items []protocol.CompletionItem
	for _, cd := range ti.Columns {
		items = append(items, item(cd.Col.Name.Name(), protocol.CompletionItemKindField, cd.Col.Type.String()))
	}
	return items
}

func (d *Document) typeItems() []protocol.CompletionItem {
	items := keywordItemsKind(builtinTypes, protocol.CompletionItemKindStruct)
	seen := map[string]bool{}
	for _, info := range d.schemas() {
		for _, ti := range info.Types {
			if label := ti.Decl.Name.String(); !seen[label] {
				seen[label] = true
				items = append(items, item(label, protocol.CompletionItemKindClass, "type"))
			}
		}
	}
	return items
}

func (d *Document) partialItems() []protocol.CompletionItem {
	var items []protocol.CompletionItem
	for _, pi := range d.Info.Partials {
		items = append(items, item(pi.Decl.Name.Name(), protocol.CompletionItemKindInterface, "table_partial"))
	}
	return items
}

func (d *Document) partialInjectionItems() []protocol.CompletionItem {
	var items []protocol.CompletionItem
	for _, pi := range d.Info.Partials {
		items = append(items, item("~"+pi.Decl.Name.Name(), protocol.CompletionItemKindInterface, "inject table_partial"))
	}
	return items
}

func keywordItems(words []string) []protocol.CompletionItem {
	return keywordItemsKind(words, protocol.CompletionItemKindKeyword)
}

func keywordItemsKind(words []string, kind protocol.CompletionItemKind) []protocol.CompletionItem {
	items := make([]protocol.CompletionItem, 0, len(words))
	for _, w := range words {
		items = append(items, item(w, kind, ""))
	}
	return items
}

func item(label string, kind protocol.CompletionItemKind, detail string) protocol.CompletionItem {
	k := kind
	it := protocol.CompletionItem{Label: label, Kind: &k}
	if detail != "" {
		det := detail
		it.Detail = &det
	}
	return it
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
