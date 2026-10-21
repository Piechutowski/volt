package lsp

import (
	"regexp"
	"strings"

	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/Piechutowski/volt/lang/check"
)

var (
	elementKeywords = []string{
		"Project", "Table", "TablePartial", "Enum", "Type", "Ref", "TableGroup",
		"Note", "Records", "DiagramView", "Migration",
		"package ", "import", "Pipeline", "Scope",
	}
	tableBodyKeywords     = []string{"indexes", "validators", "Note", "Records"}
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
		"column":    {"pk", "null", "unique", "increment", "required", "default: ", "note: ", "ref: ", "tag: "},
		"table":     {"headercolor: ", "note: "},
		"index":     {"type: ", "name: ", "unique", "pk", "where: ", "note: "},
		"migration": {"irreversible", "no transaction", "note: "},
		"ref":       {"delete: ", "update: ", "color: ", "inactive"},
		"enumvalue": {"note: "},
		"sticky":    {"color: "},
		"group":     {"note: ", "color: "},
		"scope":     {"pipe: ", "error_handler: ", "name: "},
		"route":     {"name: "},
		"resources": {"only: (", "except: (", "param: ", "singular: ", "api", "default"},
		"dataset":   {"strip: ", "only: (", "except: ("},

		"migrationcolumn": {"pk", "null", "unique", "increment", "default: ", "ref: ", "delete: ", "update: ", "note: "},
		"rebuildcolumn":   {"pk", "null", "unique", "increment", "default: ", "ref: ", "delete: ", "update: ", "note: ", "from: "},
		"migrationindex":  {"unique", "pk", "name: ", "where: ", "note: "},
		"standaloneindex": {"unique", "name: ", "where: ", "note: "},
	}

	builtinTypes = check.Builtins

	refActionValues = []string{"cascade", "restrict", "set null", "set default", "no action"}

	voltVerbSet = map[string]bool{
		"get": true, "post": true, "put": true, "patch": true,
		"delete": true, "options": true, "head": true, "any": true,
	}

	partialInjectRE = regexp.MustCompile(`~\s*[\w"]*$`)
	dotChainRE      = regexp.MustCompile(`([\p{L}\p{M}\d_]+|"[^"]*")(\.([\p{L}\p{M}\d_]+|"[^"]*"))?\.$`)
	refValueRE      = regexp.MustCompile(`(?i)\bref\s*:\s*(<>|[<>-])?\s*$`)
	afterCardRE     = regexp.MustCompile(`(<>|[<>-])\s*$`)
	defaultValueRE  = regexp.MustCompile(`(?i)\bdefault\s*:\s*$`)
	defaultChainRE  = regexp.MustCompile(`(?i)\bdefault\s*:\s*\S+\.$`)
	actionValueRE   = regexp.MustCompile(`(?i)\b(delete|update)\s*:\s*\w*$`)
	columnShapeRE   = regexp.MustCompile(`^\s*([\p{L}\p{M}\d_]+|"[^"]*")\s+[\p{L}\p{M}\d_".]*$`)
	lineStartRE     = regexp.MustCompile(`^\s*[\p{L}\p{M}\d_"~]*$`)
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
		return d.settingsComplete(prefix, ctx, outer)
	}

	if m := dotChainRE.FindStringSubmatch(prefix); m != nil {
		return d.dotChainComplete(m)
	}

	trimmed := strings.TrimSpace(prefix)
	if ctx == "ref" || strings.HasPrefix(strings.ToLower(trimmed), "ref") && strings.Contains(trimmed, ":") ||
		afterCardRE.MatchString(prefix) {
		return d.tableItems()
	}

	switch ctx {
	case "":
		if lineStartRE.MatchString(prefix) {
			return keywordItems(elementKeywords)
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
	case "tablegroup", "diagramview":
		if lineStartRE.MatchString(prefix) {
			return d.tableItems()
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
		head := headContext(strings.ToLower(strings.TrimSpace(text)))
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
	case strings.HasPrefix(head, "tablepartial"):
		return "tablepartial"
	case strings.HasPrefix(head, "tablegroup"):
		return "tablegroup"
	case strings.HasPrefix(head, "table"):
		return "table"
	case strings.HasPrefix(head, "enum"):
		return "enum"
	case strings.HasPrefix(head, "project"):
		return "project"
	case strings.HasPrefix(head, "records"):
		return "records"
	case strings.HasPrefix(head, "ref"):
		return "ref"
	case strings.HasPrefix(head, "diagramview"):
		return "diagramview"
	case strings.HasPrefix(head, "note"):
		return "note"
	case strings.HasPrefix(head, "pipeline"):
		return "pipeline"
	case strings.HasPrefix(head, "scope"):
		return "scope"
	case strings.HasPrefix(head, "migration"), upDownHeadRE.MatchString(head):
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

func (d *Document) settingsComplete(prefix, ctx, outer string) []protocol.CompletionItem {
	segment := prefix[strings.LastIndexAny(prefix, "[,")+1:]

	if refValueRE.MatchString(segment) || afterCardRE.MatchString(segment) {
		return d.tableItems()
	}
	if defaultChainRE.MatchString(segment) {
		if m := dotChainRE.FindStringSubmatch(segment); m != nil {
			return d.dotChainComplete(m)
		}
	}
	if defaultValueRE.MatchString(segment) {
		items := d.enumItems()
		items = append(items, keywordItemsKind([]string{"true", "false", "null"}, protocol.CompletionItemKindValue)...)
		return items
	}
	if actionValueRE.MatchString(segment) {
		return keywordItemsKind(refActionValues, protocol.CompletionItemKindValue)
	}

	kind := "column"
	head := strings.ToLower(strings.TrimSpace(prefix))
	verb := strings.SplitN(head, " ", 2)[0]
	switch {
	case strings.HasPrefix(head, "table") && !strings.HasPrefix(head, "tablegroup"):
		kind = "table"
	case strings.HasPrefix(head, "tablegroup"):
		kind = "group"
	case strings.HasPrefix(head, "ref"):
		kind = "ref"
	case strings.HasPrefix(head, "note"):
		kind = "sticky"
	case strings.HasPrefix(head, "scope") && (ctx == "" || ctx == "scope"):
		kind = "scope"
	case strings.HasPrefix(head, "migration") && ctx == "":
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
		case "enum":
			kind = "enumvalue"
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

func (d *Document) dotChainComplete(m []string) []protocol.CompletionItem {
	first := unquote(m[1])
	second := ""
	if m[3] != "" {
		second = unquote(m[3])
	}
	var items []protocol.CompletionItem

	if second != "" {
		if ti, ok := d.Index.Tables[first+"."+second]; ok {
			items = append(items, d.columnItems(ti)...)
		}
		return items
	}

	if ti, ok := d.Index.Tables["alias:"+first]; ok {
		items = append(items, d.columnItems(ti)...)
	}
	if ti, ok := d.Index.Tables["public."+first]; ok {
		items = append(items, d.columnItems(ti)...)
	}
	if ei, ok := d.Index.Enums["public."+first]; ok {
		for _, v := range ei.Decl.Values {
			items = append(items, item(v.Name.Name(), protocol.CompletionItemKindEnumMember, "value of "+ei.Decl.Name.String()))
		}
	}
	for key, ti := range d.Index.Tables {
		if strings.HasPrefix(key, first+".") {
			items = append(items, item(ti.Decl.Name.Base(), protocol.CompletionItemKindStruct, "Table "+key))
		}
	}
	for key, ei := range d.Index.Enums {
		if strings.HasPrefix(key, first+".") {
			items = append(items, item(ei.Decl.Name.Base(), protocol.CompletionItemKindEnum, "Enum "+key))
		}
	}
	return items
}

func (d *Document) tableItems() []protocol.CompletionItem {
	var items []protocol.CompletionItem
	seen := map[string]bool{}
	for _, ti := range d.Info.Tables {
		label := ti.Decl.Name.String()
		if !seen[label] {
			seen[label] = true
			items = append(items, item(label, protocol.CompletionItemKindStruct, "Table"))
		}
		if ti.Alias != "" && !seen[ti.Alias] {
			seen[ti.Alias] = true
			items = append(items, item(ti.Alias, protocol.CompletionItemKindStruct, "alias of "+label))
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

func (d *Document) enumItems() []protocol.CompletionItem {
	var items []protocol.CompletionItem
	for _, ei := range d.Info.Enums {
		items = append(items, item(ei.Decl.Name.String(), protocol.CompletionItemKindEnum, "Enum"))
	}
	return items
}

func (d *Document) typeItems() []protocol.CompletionItem {
	items := keywordItemsKind(builtinTypes, protocol.CompletionItemKindStruct)
	items = append(items, d.enumItems()...)
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

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
