package ast

// Inspect traverses the AST in depth-first order, calling f for each node.
// If f returns false, children of the node are not visited. It plays the
// role of go/ast's Inspect for the vet analyzers.
func Inspect(n Node, f func(Node) bool) {
	if n == nil || !f(n) {
		return
	}
	eachChild(n, func(c Node) { Inspect(c, f) })
}

// child visits a pointer field's node when the field is set. The nil
// test is on the pointer, before it becomes an interface, so a field a
// broken parse left unset never reaches a visitor as a typed nil (D89).
func child[T any, P interface {
	*T
	Node
}](visit func(Node), p P) {
	if p != nil {
		visit(p)
	}
}

// children is child over a slice of pointer fields.
func children[T any, P interface {
	*T
	Node
}](visit func(Node), ps []P) {
	for _, p := range ps {
		child(visit, p)
	}
}

// eachChild visits n's children in source order without building a
// list: a walk over a million-node file allocates nothing (D81). A
// slot typed as an interface (a declaration, a body item, an index
// key, a record value, a setting value) holds nil or a node the parser
// built from a non-nil pointer, never a typed nil, so the interface
// test is exact there.
func eachChild(n Node, visit func(Node)) {
	switch n := n.(type) {
	case *File:
		for _, d := range n.Decls {
			if d != nil {
				visit(d)
			}
		}
	case *Use:
		children(visit, n.Items)
		child(visit, n.Path)
	case *UseItem:
		child(visit, n.Kind)
		child(visit, n.Name)
		child(visit, n.Alias)
	case *Project:
		child(visit, n.Name)
		children(visit, n.Props)
		children(visit, n.Notes)
	case *ProjectProp:
		child(visit, n.Key)
		child(visit, n.Value)
	case *Table:
		child(visit, n.Name)
		child(visit, n.Alias)
		child(visit, n.Settings)
		for _, it := range n.Body {
			if it != nil {
				visit(it)
			}
		}
	case *TablePartial:
		child(visit, n.Name)
		child(visit, n.Settings)
		for _, it := range n.Body {
			if it != nil {
				visit(it)
			}
		}
	case *Column:
		child(visit, n.Name)
		child(visit, n.Type)
		child(visit, n.Settings)
		children(visit, n.LegacyFlags)
	case *TypeRef:
		child(visit, n.Name)
	case *IndexesBlock:
		children(visit, n.Indexes)
	case *Index:
		for _, k := range n.Key {
			if k != nil {
				visit(k)
			}
		}
		child(visit, n.Settings)
	case *ChecksBlock:
		children(visit, n.Checks)
	case *Check:
		child(visit, n.Expr)
		children(visit, n.Args)
		child(visit, n.Settings)
	case *PartialRef:
		child(visit, n.Name)
	case *Note:
		child(visit, n.Text)
	case *Ref:
		child(visit, n.Name)
		child(visit, n.Left)
		child(visit, n.Right)
		child(visit, n.Settings)
	case *RefEndpoint:
		child(visit, n.Table)
		children(visit, n.Columns)
	case *Enum:
		child(visit, n.Name)
		children(visit, n.Values)
	case *EnumValue:
		child(visit, n.Name)
		child(visit, n.Settings)
	case *Records:
		child(visit, n.Table)
		children(visit, n.Columns)
		children(visit, n.Rows)
	case *RecordRow:
		for _, v := range n.Values {
			if v != nil {
				visit(v)
			}
		}
	case *StickyNote:
		child(visit, n.Name)
		child(visit, n.Settings)
		child(visit, n.Text)
	case *TableGroup:
		child(visit, n.Name)
		child(visit, n.Settings)
		children(visit, n.Members)
		children(visit, n.Notes)
	case *DiagramView:
		child(visit, n.Name)
		children(visit, n.Categories)
	case *ViewCategory:
		child(visit, n.Kind)
		children(visit, n.Names)
	case *SettingList:
		children(visit, n.Settings)
	case *Setting:
		if n.Value != nil {
			visit(n.Value)
		}
	case *QualName:
		children(visit, n.Parts)
	case *NegNumber:
		child(visit, n.Num)
	case *EnumConst:
		child(visit, n.Enum)
		child(visit, n.Value)
	case *RefValue:
		child(visit, n.Endpoint)
	}
}
