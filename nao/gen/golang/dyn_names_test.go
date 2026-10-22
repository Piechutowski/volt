package golang

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
		size += 5 + len(dynWrapperSuffixes) + len(tm.fields)
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
		nc := NameCollision{Name: name, First: first.describe(), Second: second.describe(), Pos: second.pos}
		if first.kind == dynType {
			nc.Type, nc.With = first.td, second.describe()
		} else if second.kind == dynType {
			nc.Type, nc.With = second.td, first.describe()
		}
		out = append(out, nc)
	}

	for _, td := range info.Types {
		if td.Path == "" {
			add(td.Key, dynOrigin{kind: dynType, td: td, pos: td.Decl.Name.Pos()})
		}
	}
	for _, tm := range p.tables {
		pos := tm.ti.Decl.Pos()
		add(tm.model, dynOrigin{kind: dynModel, tm: tm, pos: pos})
		if len(tm.fields) == 0 {
			continue
		}
		add(tm.model+"Changeset", dynOrigin{kind: dynChangeset, tm: tm, pos: pos})
		add(tm.model+"Column", dynOrigin{kind: dynColumn, tm: tm, pos: pos})
		add(tm.model+"Params", dynOrigin{kind: dynParams, tm: tm, pos: pos})
		add(tm.model+"New", dynOrigin{kind: dynNew, tm: tm, pos: pos})
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
		fmt.Fprintf(&b, "%s: %s | %s | %s", c.Pos, c.Name, c.First, c.Second)
		if c.Type != nil {
			fmt.Fprintf(&b, " | Type %s with %s", c.Type.Key, c.With)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func TestDynNameCollisionsMatchWalk(t *testing.T) {
	base := `Table users {
  id int [pk]
  foo_bar int
  limit int
  status OrderState
}
Type OrderState string
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
Type EUserKind string
Type EUserX string
Type UserFooBar string
Table pinned [model: "UserChangeset"] {
  id int [pk]
  n int
}
Table pinned_update [model: "UserNew"] {
  id int [pk]
  n int
}
Table user_columns {
  id int [pk]
}
Table user_params {
  id int [pk]
}
Table nokey {
  a int
  b int
}
Table allgen {
  id int [pk, increment]
}
Table user_offsets [model: "UserOffset"] {
  id int [pk]
}
`
	edits := []string{
		base,
		strings.Replace(base, "Table user_foo_bars {", "Table user_foo_bars_x {", 1),
		strings.Replace(base, "  bar int\n", "  baz int\n", 1),
		strings.Replace(base, "Table allgen {", "Table allgen [model: \"UserSet\"] {", 1),
		strings.Replace(base, "  n int\n}\nTable pinned_update", "  n int\n  m int\n}\nTable pinned_update", 1),
		base,
	}
	var reuse *parser.Reuse
	var schema check.Memo
	var memo PlanMemo
	for i, src := range edits {
		f, diags, next, _ := parser.ParseFileReuse("dyn.volt", src, reuse)
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
	f, _ := parser.ParseFile("dyn.volt", base)
	info, _ := check.File(f)
	first := collisionsRender(PlanBuild(f, info).DynNameCollisions())
	for i := 0; i < 20; i++ {
		if again := collisionsRender(PlanBuild(f, info).DynNameCollisions()); again != first {
			t.Fatalf("run %d differs:\n%s\n---\n%s", i, again, first)
		}
	}
	_ = sort.Strings
}
