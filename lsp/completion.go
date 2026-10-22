package lsp

import (
	"regexp"
	"strings"

	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/parser"
)

var (
	elementKeywords = []string{
		"Project", "Table", "TablePartial", "Type", "Ref",
		"Records", "Migration",
		"package ", "import", "Pipeline", "Scope",
		"Group", "Pred", "Select",
	}
	tableBodyKeywords     = []string{"Indexes", "Validators", "Records"}
	migrationBodyKeywords = []string{
		"create table ", "create index ", "drop table ", "drop index ",
		"rename table ", "alter table ", "rebuild table ", "exec ", "up {", "down {",
	}
	alterBodyKeywords = []string{"add ", "drop ", "rename "}
	scopeBodyKeywords = []string{
		"get ", "post ", "put ", "patch ", "delete ", "options ", "head ",
		"any ", "resources ", "dataset ", "Scope ",
	}

	settingsByContext = map[string][]string{
		"column":    {"pk", "null", "unique", "index", "increment", "required", "default: ", "note: ", "ref: ", "tag: "},
		"project":   {"note: "},
		"table":     {"note: ", "model: "},
		"select":    {"order: ("},
		"index":     {"name: ", "unique", "where: "},
		"migration": {"irreversible", "no_transaction"},
		"ref":       {"delete: ", "update: "},
		"scope":     {"pipe: ", "error_handler: ", "name: "},
		"route":     {"name: "},
		"resources": {"only: (", "except: (", "param: ", "singular: ", "api", "default"},
		"dataset":   {"strip: ", "only: (", "except: ("},

		"migrationcolumn": {"pk", "null", "unique", "index", "increment", "default: ", "ref: ", "delete: ", "update: "},
		"rebuildcolumn":   {"pk", "null", "unique", "index", "increment", "default: ", "ref: ", "delete: ", "update: ", "from: "},
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

	partialInjectRE = regexp.MustCompile(`~\s*\w*$`)
	dotChainRE      = regexp.MustCompile(`([\p{L}\p{Nd}_]+)(\.([\p{L}\p{Nd}_]+))?\.$`)
	refValueRE      = regexp.MustCompile(`\bref\s*:\s*$`)
	refDotRE        = regexp.MustCompile(`\bref\s*:\s*([\p{L}\p{Nd}_]+)\.$`)
	refHeadRE       = regexp.MustCompile(`^\s*Ref\s*:\s*[\p{L}\p{Nd}_]*$`)
	refTargetRE     = regexp.MustCompile(`^\s*Ref\s*:[^>]*>\s*[\p{L}\p{Nd}_]*$`)
	defaultValueRE  = regexp.MustCompile(`\bdefault\s*:\s*$`)
	columnTypeRE    = regexp.MustCompile(`^\s*(?:add\s+)?[\p{L}\p{Nd}_]+\s+([\p{L}\p{Nd}_.\[\]]+)\s*\[`)
	actionValueRE   = regexp.MustCompile(`\b(delete|update)\s*:\s*\w*$`)
	columnShapeRE   = regexp.MustCompile(`^\s*([\p{L}\p{Nd}_]+)\s+[\p{L}\p{Nd}_.]*$`)
	lineStartRE     = regexp.MustCompile(`^\s*[\p{L}\p{Nd}_~]*$`)
	shapeHeadRE     = regexp.MustCompile(`^(create|rebuild|drop) table\b`)
	upDownHeadRE    = regexp.MustCompile(`^(up|down)\s*\{`)
)

// Complete computes completion items for the position.
func (d *Document) Complete(pos protocol.Position) []protocol.CompletionItem {
	offset := d.FromLSP(pos)
	lineStart := d.lineOffsets[min(int(pos.Line), len(d.lineOffsets)-1)]
	prefix := d.Text[lineStart:offset]
	ctx, outer := d.blockContexts(int(pos.Line))

	if items := d.voltComplete(prefix); items != nil {
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
			return keywordItems([]string{"Indexes"})
		}
	}
	return nil
}

func (d *Document) blockContexts(line int) (ctx, outer string) {
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
	case strings.HasPrefix(head, "Indexes"):
		return "indexes"
	case strings.HasPrefix(head, "Validators"):
		return "validators"
	case strings.HasPrefix(head, "TablePartial"):
		return "tablepartial"
	case strings.HasPrefix(head, "Group"):
		return "group"
	case strings.HasPrefix(head, "Table"):
		return "table"
	case strings.HasPrefix(head, "Project"):
		return "project"
	case strings.HasPrefix(head, "Records"):
		return "records"
	case strings.HasPrefix(head, "Ref"):
		return "ref"
	case strings.HasPrefix(head, "Pipeline"):
		return "pipeline"
	case strings.HasPrefix(head, "Scope"):
		return "scope"
	case strings.HasPrefix(head, "Migration"), upDownHeadRE.MatchString(head):
		return "migration"
	case strings.HasPrefix(head, "rebuild table"):
		return "rebuild"
	case shapeHeadRE.MatchString(head):
		return "shape"
	case strings.HasPrefix(head, "alter table"):
		return "alter"
	}
	return ""
}

func (d *Document) settingsComplete(prefix, ctx, outer string, top bool) []protocol.CompletionItem {
	segment := prefix[strings.LastIndexAny(prefix, "[,")+1:]

	if refValueRE.MatchString(segment) {
		return d.tableItems()
	}
	if m := refDotRE.FindStringSubmatch(segment); m != nil {
		if ti, ok := d.Index.Tables[m[1]]; ok {
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
	case header("TablePartial"), header("Group"), header("Type "):
		return nil
	case header("Table"):
		kind = "table"
	case header("Project"):
		kind = "project"
	case header("Ref"):
		kind = "ref"
	case header("Select"):
		kind = "select"
	case strings.HasPrefix(head, "Scope") && (top || ctx == "scope"):
		kind = "scope"
	case header("Migration"):
		kind = "migration"
	case strings.HasPrefix(head, "resources") && ctx == "scope":
		kind = "resources"
	case strings.HasPrefix(head, "dataset") && ctx == "scope":
		kind = "dataset"
	case voltVerbSet[verb] && ctx == "scope":
		kind = "route"
	case (strings.HasPrefix(head, "create index") || strings.HasPrefix(head, "drop index")) && ctx == "migration":
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
		return true
	}
	if m[1] == "bool" {
		return true
	}
	for _, ti := range d.Info.Types {
		if ti.Key == m[1] {
			return ti.Storage.Storage == "bool"
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
	if ti, ok := d.Index.Tables[first]; ok {
		items = append(items, d.columnItems(ti)...)
	}
	return items
}

func (d *Document) tableItems() []protocol.CompletionItem {
	var items []protocol.CompletionItem
	seen := map[string]bool{}
	for _, ti := range d.Info.Tables {
		label := ti.Decl.Name.Name()
		if !seen[label] {
			seen[label] = true
			items = append(items, item(label, protocol.CompletionItemKindStruct, "Table"))
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
	for _, ti := range d.Info.Types {
		items = append(items, item(ti.Decl.Name.String(), protocol.CompletionItemKindClass, "Type"))
	}
	return items
}

func (d *Document) partialItems() []protocol.CompletionItem {
	var items []protocol.CompletionItem
	for _, pi := range d.Info.Partials {
		items = append(items, item(pi.Decl.Name.Name(), protocol.CompletionItemKindInterface, "TablePartial"))
	}
	return items
}

func (d *Document) partialInjectionItems() []protocol.CompletionItem {
	var items []protocol.CompletionItem
	for _, pi := range d.Info.Partials {
		items = append(items, item("~"+pi.Decl.Name.Name(), protocol.CompletionItemKindInterface, "inject TablePartial"))
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
