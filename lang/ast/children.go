package ast

// Children lists n's children in source order, the same children
// Inspect visits, as a slice: for a walk written without function
// values, which a memoized computation may not call (D102). The list
// is built by a type switch of its own, kept equal to eachChild's by
// test, because eachChild visits through a function value and so
// cannot serve one.
func Children(n Node) []Node {
	var out []Node
	switch n := n.(type) {
	case *File:
		for _, d := range n.Decls {
			if d != nil {
				out = append(out, d)
			}
		}
	case *Use:
		for _, it := range n.Items {
			if it != nil {
				out = append(out, it)
			}
		}
		if n.Path != nil {
			out = append(out, n.Path)
		}
	case *UseItem:
		if n.Kind != nil {
			out = append(out, n.Kind)
		}
		if n.Name != nil {
			out = append(out, n.Name)
		}
		if n.Alias != nil {
			out = append(out, n.Alias)
		}
	case *Project:
		if n.Name != nil {
			out = append(out, n.Name)
		}
		for _, p := range n.Props {
			if p != nil {
				out = append(out, p)
			}
		}
		for _, x := range n.Notes {
			if x != nil {
				out = append(out, x)
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
		if n.Alias != nil {
			out = append(out, n.Alias)
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
		if n.Settings != nil {
			out = append(out, n.Settings)
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
		for _, f := range n.LegacyFlags {
			if f != nil {
				out = append(out, f)
			}
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
	case *ChecksBlock:
		for _, c := range n.Checks {
			if c != nil {
				out = append(out, c)
			}
		}
	case *Check:
		if n.Expr != nil {
			out = append(out, n.Expr)
		}
		for _, a := range n.Args {
			if a != nil {
				out = append(out, a)
			}
		}
		if n.Settings != nil {
			out = append(out, n.Settings)
		}
	case *PartialRef:
		if n.Name != nil {
			out = append(out, n.Name)
		}
	case *Note:
		if n.Text != nil {
			out = append(out, n.Text)
		}
	case *Ref:
		if n.Name != nil {
			out = append(out, n.Name)
		}
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
	case *Enum:
		if n.Name != nil {
			out = append(out, n.Name)
		}
		for _, v := range n.Values {
			if v != nil {
				out = append(out, v)
			}
		}
	case *EnumValue:
		if n.Name != nil {
			out = append(out, n.Name)
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
	case *StickyNote:
		if n.Name != nil {
			out = append(out, n.Name)
		}
		if n.Settings != nil {
			out = append(out, n.Settings)
		}
		if n.Text != nil {
			out = append(out, n.Text)
		}
	case *TableGroup:
		if n.Name != nil {
			out = append(out, n.Name)
		}
		if n.Settings != nil {
			out = append(out, n.Settings)
		}
		for _, m := range n.Members {
			if m != nil {
				out = append(out, m)
			}
		}
		for _, x := range n.Notes {
			if x != nil {
				out = append(out, x)
			}
		}
	case *DiagramView:
		if n.Name != nil {
			out = append(out, n.Name)
		}
		for _, c := range n.Categories {
			if c != nil {
				out = append(out, c)
			}
		}
	case *ViewCategory:
		if n.Kind != nil {
			out = append(out, n.Kind)
		}
		for _, x := range n.Names {
			if x != nil {
				out = append(out, x)
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
	case *EnumConst:
		if n.Enum != nil {
			out = append(out, n.Enum)
		}
		if n.Value != nil {
			out = append(out, n.Value)
		}
	case *RefValue:
		if n.Endpoint != nil {
			out = append(out, n.Endpoint)
		}
	}
	return out
}
