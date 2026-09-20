package golang

// DynNameCollisions is computed from the collisions alone, which the
// name base keeps across plans (D104). The oracle is the walk it
// replaced: every minted name in minting order through one map. The
// two must agree on every input, edit after edit through the memo.

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Piechutowski/volt/lang/check"
	"github.com/Piechutowski/volt/lang/parser"
)

func dynNamesWalk(p *plan, info *check.Info) []NameCollision {
	size := 1
	for _, tm := range p.tables {
		size += 3 + len(dynWrapperSuffixes) + len(tm.fields)
	}
	seen := make(map[string]dynOrigin, size)
	seen["Queries"] = dynOrigin{kind: dynQueries}
	var out []NameCollision
	add := func(name string, o dynOrigin) {
		prev, dup := seen[name]
		if !dup {
			seen[name] = o
			return
		}
		first, second := prev, o
		if second.pos.Line() < first.pos.Line() || (second.pos.Line() == first.pos.Line() && second.pos.Column() < first.pos.Column()) {
			first, second = second, first
		}
		out = append(out, NameCollision{Name: name, First: first.describe(), Second: second.describe(), Pos: second.pos})
	}

	for _, e := range info.Enums {
		typeName, err := enumTypeName(e.Decl.Name.Schema(), e.Decl.Name.Base())
		if err != nil {
			continue // generation reports unusable names itself
		}
		add(typeName, dynOrigin{kind: dynEnum, e: e, pos: e.Decl.Pos()})
		for _, v := range e.Decl.Values {
			constName, err := goName(v.Name.Name())
			if err != nil {
				continue
			}
			add(typeName+constName, dynOrigin{kind: dynEnumValue, e: e, v: v, pos: v.Pos()})
		}
	}
	for _, tm := range p.tables {
		pos := tm.ti.Decl.Pos()
		add(tm.model, dynOrigin{kind: dynModel, tm: tm, pos: pos})
		if len(tm.fields) == 0 {
			continue // no queryable shape: no queries, no dynamic layer
		}
		if len(tm.createFields()) > 0 {
			add(tm.model+"CreateParams", dynOrigin{kind: dynCreateParams, tm: tm, pos: pos})
		}
		if len(tm.pk) > 0 && len(tm.nonPK()) > 0 {
			add(tm.model+"UpdateParams", dynOrigin{kind: dynUpdateParams, tm: tm, pos: pos})
		}
		for _, f := range tm.fields {
			add(tm.model+f.goField, dynOrigin{kind: dynHandle, tm: tm, f: f, pos: f.col.Pos()})
		}
		for _, sfx := range dynWrapperSuffixes {
			add(tm.model+sfx, dynOrigin{kind: dynWrapper, tm: tm, sfx: sfx, pos: pos})
		}
	}
	return out
}

func collisionsRender(cs []NameCollision) string {
	var b strings.Builder
	for _, c := range cs {
		fmt.Fprintf(&b, "%s: %s | %s | %s\n", c.Pos, c.Name, c.First, c.Second)
	}
	return b.String()
}

func TestDynNameCollisionsMatchWalk(t *testing.T) {
	base := `Table users {
  id int [pk]
  foo_bar int
  limit int
  status order_status [not null]
}
Enum order_status {
  pending
}
Table user_limits {
  id int [pk]
}
Table user_foos {
  id int [pk]
  bar int
}
Table user_foo_bars {
  id int [pk]
}
Table e_users {
  id int [pk]
  kind int
  x int
}
Enum user_kind {
  x
}
Enum e_user {
  x
}
Table pinned [model: 'UserCreateParams'] {
  id int [pk]
  n int
}
Table pinned_update [model: 'UserUpdateParams'] {
  id int [pk]
  n int
}
Table nokey {
  a int
  b int
}
Table allgen {
  id int [pk, increment]
}
Table user_offsets [model: 'UserOffset'] {
  id int [pk]
}
`
	// edits: one adds a collision, one removes one, one renames a
	// model, one touches an unrelated table
	edits := []string{
		base,
		strings.Replace(base, "Table user_foo_bars {", "Table user_foo_bars_x {", 1),
		strings.Replace(base, "  bar int\n", "  baz int\n", 1),
		strings.Replace(base, "Table allgen {", "Table allgen [model: 'UserSet'] {", 1),
		strings.Replace(base, "  n int\n}\nTable pinned_update", "  n int\n  m int\n}\nTable pinned_update", 1),
		base,
	}
	var reuse *parser.Reuse
	var schema check.Memo
	var memo PlanMemo
	for i, src := range edits {
		f, diags, next, _ := parser.ParseFileReuse("dyn.dbml", src, reuse)
		reuse = next
		if len(diags) > 0 {
			t.Fatalf("edit %d: %v", i, diags)
		}
		info, semDiags := check.FileMemo(f, &schema)
		if len(semDiags) > 0 {
			t.Fatalf("edit %d: %v", i, semDiags)
		}
		pl := PlanBuildMemo(f, info, &memo)
		if pl.err != nil {
			t.Fatalf("edit %d: plan: %v", i, pl.err)
		}
		got := collisionsRender(pl.DynNameCollisions())
		want := collisionsRender(dynNamesWalk(pl.p, pl.info))
		if got != want {
			t.Fatalf("edit %d: the base's report differs from the walk's\n--- base\n%s--- walk\n%s", i, got, want)
		}
		if i == 0 && strings.Count(got, "\n") < 8 {
			t.Fatalf("the fixture exercises too little:\n%s", got)
		}
		fresh := PlanBuild(f, info)
		if fromFresh := collisionsRender(fresh.DynNameCollisions()); fromFresh != got {
			t.Fatalf("edit %d: the memoized plan's report differs from a fresh plan's\n--- memo\n%s--- fresh\n%s", i, got, fromFresh)
		}
	}
	// The report's order is total, whatever order the maps give.
	f, _ := parser.ParseFile("dyn.dbml", base)
	info, _ := check.File(f)
	first := collisionsRender(PlanBuild(f, info).DynNameCollisions())
	for i := 0; i < 20; i++ {
		if again := collisionsRender(PlanBuild(f, info).DynNameCollisions()); again != first {
			t.Fatalf("run %d differs:\n%s\n---\n%s", i, again, first)
		}
	}
	_ = sort.Strings
}
