package ast

// Children lists n's children in source order, the same children
// Inspect visits, as a slice: for a walk written without function
// values, which a memoized computation may not call.
func Children(n Node) []Node {
	var out []Node
	switch n := n.(type) {
	case *File:
		for _, d := range n.Decls {
			if d != nil {
				out = append(out, d)
			}
		}
	case *Project:
		if n.Name != nil {
			out = append(out, n.Name)
		}
		if n.Settings != nil {
			out = append(out, n.Settings)
		}
		for _, p := range n.Props {
			if p != nil {
				out = append(out, p)
			}
		}
	case *ProjectProp:
		if n.Key != nil {
			out = append(out, n.Key)
		}
		if n.Value != nil {
			out = append(out, n.Value)
		}
	case *Table:
		if n.Name != nil {
			out = append(out, n.Name)
		}
		if n.Settings != nil {
			out = append(out, n.Settings)
		}
		for _, it := range n.Body {
			if it != nil {
				out = append(out, it)
			}
		}
	case *TablePartial:
		if n.Name != nil {
			out = append(out, n.Name)
		}
		for _, it := range n.Body {
			if it != nil {
				out = append(out, it)
			}
		}
	case *Column:
		if n.Name != nil {
			out = append(out, n.Name)
		}
		if n.Type != nil {
			out = append(out, n.Type)
		}
		if n.Settings != nil {
			out = append(out, n.Settings)
		}
	case *TypeRef:
		if n.Name != nil {
			out = append(out, n.Name)
		}
	case *IndexesBlock:
		for _, ix := range n.Indexes {
			if ix != nil {
				out = append(out, ix)
			}
		}
	case *Index:
		for _, k := range n.Key {
			if k != nil {
				out = append(out, k)
			}
		}
		if n.Settings != nil {
			out = append(out, n.Settings)
		}
	case *ValidatorsBlock:
		for _, v := range n.Validators {
			if v != nil {
				out = append(out, v)
			}
		}
	case *Validator:
		if n.Ref != nil {
			out = append(out, n.Ref)
		}
		for _, a := range n.Args {
			if a != nil {
				out = append(out, a)
			}
		}
	case *PartialRef:
		if n.Name != nil {
			out = append(out, n.Name)
		}
	case *Ref:
		if n.Left != nil {
			out = append(out, n.Left)
		}
		if n.Right != nil {
			out = append(out, n.Right)
		}
		if n.Settings != nil {
			out = append(out, n.Settings)
		}
	case *RefEndpoint:
		if n.Table != nil {
			out = append(out, n.Table)
		}
		for _, c := range n.Columns {
			if c != nil {
				out = append(out, c)
			}
		}
	case *TypeDecl:
		if n.Name != nil {
			out = append(out, n.Name)
		}
		if n.Storage != nil {
			out = append(out, n.Storage)
		}
		if n.From != nil {
			out = append(out, n.From)
		}
	case *Migration:
		if n.Name != nil {
			out = append(out, n.Name)
		}
		if n.Settings != nil {
			out = append(out, n.Settings)
		}
		for _, op := range n.Up {
			if op != nil {
				out = append(out, op)
			}
		}
		for _, op := range n.Down {
			if op != nil {
				out = append(out, op)
			}
		}
	case *CreateTable:
		if n.Name != nil {
			out = append(out, n.Name)
		}
		for _, it := range n.Body {
			if it != nil {
				out = append(out, it)
			}
		}
	case *DropTable:
		if n.Name != nil {
			out = append(out, n.Name)
		}
		for _, it := range n.Body {
			if it != nil {
				out = append(out, it)
			}
		}
	case *RenameTable:
		if n.From != nil {
			out = append(out, n.From)
		}
		if n.To != nil {
			out = append(out, n.To)
		}
	case *AlterTable:
		if n.Name != nil {
			out = append(out, n.Name)
		}
		for _, it := range n.Items {
			if it != nil {
				out = append(out, it)
			}
		}
	case *AlterAdd:
		if n.Column != nil {
			out = append(out, n.Column)
		}
	case *AlterDrop:
		if n.Name != nil {
			out = append(out, n.Name)
		}
		if n.Type != nil {
			out = append(out, n.Type)
		}
		if n.Settings != nil {
			out = append(out, n.Settings)
		}
	case *AlterRename:
		if n.From != nil {
			out = append(out, n.From)
		}
		if n.To != nil {
			out = append(out, n.To)
		}
	case *RebuildTable:
		if n.Name != nil {
			out = append(out, n.Name)
		}
		for _, it := range n.Body {
			if it != nil {
				out = append(out, it)
			}
		}
	case *CreateIndex:
		if n.Table != nil {
			out = append(out, n.Table)
		}
		if n.Index != nil {
			out = append(out, n.Index)
		}
	case *DropIndex:
		if n.Table != nil {
			out = append(out, n.Table)
		}
		if n.Index != nil {
			out = append(out, n.Index)
		}
	case *Exec:
		if n.SQL != nil {
			out = append(out, n.SQL)
		}
		if n.Settings != nil {
			out = append(out, n.Settings)
		}
	case *Records:
		if n.Table != nil {
			out = append(out, n.Table)
		}
		for _, c := range n.Columns {
			if c != nil {
				out = append(out, c)
			}
		}
		for _, r := range n.Rows {
			if r != nil {
				out = append(out, r)
			}
		}
	case *RecordRow:
		for _, v := range n.Values {
			if v != nil {
				out = append(out, v)
			}
		}
	case *SettingList:
		for _, s := range n.Settings {
			if s != nil {
				out = append(out, s)
			}
		}
	case *Setting:
		if n.Value != nil {
			out = append(out, n.Value)
		}
	case *QualName:
		for _, p := range n.Parts {
			if p != nil {
				out = append(out, p)
			}
		}
	case *NegNumber:
		if n.Num != nil {
			out = append(out, n.Num)
		}
	case *QualIdent:
		if n.Qual != nil {
			out = append(out, n.Qual)
		}
		if n.Name != nil {
			out = append(out, n.Name)
		}
	}
	return out
}
