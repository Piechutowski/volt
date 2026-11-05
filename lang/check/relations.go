// Relations (spec §relationships-ref): the names a select joins by,
// derived from every resolved relationship, one per side; a name two
// relations share, or a table's own, is ambiguous, and as: and reverse:
// exist to name one outright.
package check

import (
	"sort"
	"strings"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/token"
)

// RelationInfo is one relation of a table (spec §relationships-ref): the
// name a select joins it by, the table it leads to, whether it may lead
// to no row, and the column pairs the join matches, own column first.
type RelationInfo struct {
	Name     string
	Ref      *RefInfo
	Forward  bool
	Table    *TableInfo
	Optional bool
	On       [][2]string
	Pos      token.Position
}

// Relations lists a table's relations in the order of their
// relationships (spec §relationships-ref); none for an unknown table.
func (i *Info) Relations(table string) []*RelationInfo { return i.relations[table] }

// Relation is the table's relation of that name when exactly one
// carries it and it is not the table's own; otherwise nil, with every
// relation of that name, so the caller can say why (spec
// §relationships-ref).
func (i *Info) Relation(table, name string) (rel *RelationInfo, named []*RelationInfo) {
	for _, r := range i.relations[table] {
		if strings.EqualFold(r.Name, name) {
			named = append(named, r)
		}
	}
	if len(named) == 1 && named[0].Name == name && !strings.EqualFold(name, table) {
		return named[0], named
	}
	return nil, named
}

// RelationNames lists a table's relation names, sorted, an ambiguous
// one marked.
func (i *Info) RelationNames(table string) []string {
	seen := map[string]int{}
	for _, r := range i.relations[table] {
		seen[strings.ToLower(r.Name)]++
	}
	out := make([]string, 0, len(seen))
	for _, r := range i.relations[table] {
		name := r.Name
		if seen[strings.ToLower(name)] > 1 || strings.EqualFold(name, table) {
			name += " (ambiguous)"
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Side names the relation's direction for a diagnostic.
func (r *RelationInfo) Side() string {
	if r.Forward {
		return "forward"
	}
	return "reverse"
}

func relationResolved(ri *RefInfo) bool {
	return ri.FK.Table != nil && ri.Target.Table != nil && len(ri.FK.Columns) == len(ri.Target.Columns)
}

func (c *checker) relationsBuild() {
	count := make(map[string]int, len(c.info.Tables))
	refs, pairs := 0, 0
	for _, ri := range c.info.Refs {
		if relationResolved(ri) {
			count[ri.FK.Table.Key]++
			count[ri.Target.Table.Key]++
			refs++
			pairs += len(ri.FK.Columns)
		}
	}
	c.info.relations = make(map[string][]*RelationInfo, len(count))
	if refs == 0 {
		return
	}
	rels := make([]RelationInfo, 2*refs)
	on := make([][2]string, 2*pairs)
	lists := make([]*RelationInfo, 2*refs)
	for key, n := range count {
		c.info.relations[key], lists = lists[:0:n], lists[n:]
	}
	for _, ri := range c.info.Refs {
		if !relationResolved(ri) {
			continue
		}
		n := len(ri.FK.Columns)
		forward, reverse := &rels[0], &rels[1]
		rels = rels[2:]
		*forward = RelationInfo{Ref: ri, Forward: true, Table: ri.Target.Table, Pos: ri.FK.Pos, On: on[:n:n]}
		*reverse = RelationInfo{Ref: ri, Table: ri.FK.Table, Optional: true, Pos: ri.FK.Pos, On: on[n : 2*n : 2*n]}
		on = on[2*n:]
		for i, col := range ri.FK.Columns {
			forward.On[i] = [2]string{col, ri.Target.Columns[i]}
			reverse.On[i] = [2]string{ri.Target.Columns[i], col}
			if cd := ri.FK.Table.Column(col); cd != nil && cd.Nullable {
				forward.Optional = true
			}
		}
		forward.Name = ri.Target.Table.Key
		if len(ri.FK.Columns) == 1 {
			forward.Name = RelationStem(ri.FK.Columns[0])
		}
		if as := refNameSetting(ri, "as"); as != nil {
			forward.Name = as.Name()
			forward.Pos = as.Pos()
		}
		reverse.Name = ri.FK.Table.Key
		if as := refNameSetting(ri, "reverse"); as != nil {
			reverse.Name = as.Name()
			reverse.Pos = as.Pos()
		}
		c.relationAdd(ri.FK.Table, forward)
		c.relationAdd(ri.Target.Table, reverse)
	}
}

// RelationStem is the name a forward relation over the one column col
// takes (spec §relationships-ref): col without a trailing _id, else col.
func RelationStem(col string) string {
	if stem, ok := strings.CutSuffix(col, "_id"); ok && stem != "" {
		return stem
	}
	return col
}

func refNameSetting(ri *RefInfo, name string) *ast.Ident {
	r, ok := ri.Node.(*ast.Ref)
	if !ok || r.Settings == nil {
		return nil
	}
	if s := r.Settings.Get(name); s != nil {
		if id, ok := s.Value.(*ast.Ident); ok {
			return id
		}
	}
	return nil
}

func (c *checker) relationAdd(t *TableInfo, r *RelationInfo) {
	c.info.relations[t.Key] = append(c.info.relations[t.Key], r)
}
