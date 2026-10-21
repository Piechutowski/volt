package ast

// Inspect traverses the AST in depth-first order, calling f for each node.
// If f returns false, children of the node are not visited.
func Inspect(n Node, f func(Node) bool) {
	if n == nil || !f(n) {
		return
	}
	eachChild(n, func(c Node) { Inspect(c, f) })
}

func child[T any, P interface {
	*T
	Node
}](visit func(Node), p P) {
	if p != nil {
		visit(p)
	}
}

func children[T any, P interface {
	*T
	Node
}](visit func(Node), ps []P) {
	for _, p := range ps {
		child(visit, p)
	}
}

func eachChild(n Node, visit func(Node)) {
	switch n := n.(type) {
	case *File:
		for _, d := range n.Decls {
			if d != nil {
				visit(d)
			}
		}
	case *Project:
		child(visit, n.Name)
		child(visit, n.Settings)
		children(visit, n.Props)
	case *ProjectProp:
		child(visit, n.Key)
		child(visit, n.Value)
	case *Table:
		child(visit, n.Name)
		child(visit, n.Settings)
		for _, it := range n.Body {
			if it != nil {
				visit(it)
			}
		}
	case *TablePartial:
		child(visit, n.Name)
		for _, it := range n.Body {
			if it != nil {
				visit(it)
			}
		}
	case *Column:
		child(visit, n.Name)
		child(visit, n.Type)
		child(visit, n.Settings)
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
	case *ValidatorsBlock:
		children(visit, n.Validators)
	case *Validator:
		child(visit, n.Ref)
		children(visit, n.Args)
	case *PartialRef:
		child(visit, n.Name)
	case *Ref:
		child(visit, n.Left)
		child(visit, n.Right)
		child(visit, n.Settings)
	case *RefEndpoint:
		child(visit, n.Table)
		children(visit, n.Columns)
	case *TypeDecl:
		child(visit, n.Name)
		child(visit, n.Storage)
		child(visit, n.From)
	case *Migration:
		child(visit, n.Name)
		child(visit, n.Settings)
		for _, op := range n.Up {
			if op != nil {
				visit(op)
			}
		}
		for _, op := range n.Down {
			if op != nil {
				visit(op)
			}
		}
	case *CreateTable:
		child(visit, n.Name)
		for _, it := range n.Body {
			if it != nil {
				visit(it)
			}
		}
	case *DropTable:
		child(visit, n.Name)
		for _, it := range n.Body {
			if it != nil {
				visit(it)
			}
		}
	case *RenameTable:
		child(visit, n.From)
		child(visit, n.To)
	case *AlterTable:
		child(visit, n.Name)
		for _, it := range n.Items {
			if it != nil {
				visit(it)
			}
		}
	case *AlterAdd:
		child(visit, n.Column)
	case *AlterDrop:
		child(visit, n.Name)
		child(visit, n.Type)
		child(visit, n.Settings)
	case *AlterRename:
		child(visit, n.From)
		child(visit, n.To)
	case *RebuildTable:
		child(visit, n.Name)
		for _, it := range n.Body {
			if it != nil {
				visit(it)
			}
		}
	case *CreateIndex:
		child(visit, n.Table)
		child(visit, n.Index)
	case *DropIndex:
		child(visit, n.Table)
		child(visit, n.Index)
	case *Exec:
		child(visit, n.SQL)
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
	case *QualIdent:
		child(visit, n.Qual)
		child(visit, n.Name)
	}
}
