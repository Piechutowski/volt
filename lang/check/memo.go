package check

import (
	"slices"

	"github.com/Piechutowski/volt/lang/ast"
	"github.com/Piechutowski/volt/lang/diag"
)

// Memo remembers a file's checked tables across its versions. The zero
// value is ready; a Memo belongs to one file and one goroutine at a
// time.
type Memo struct {
	prev, next map[*ast.Table]*memoTable
	// Hits and Misses count the tables answered from the memo and
	// checked afresh by the last FileMemo.
	Hits, Misses int
}

type memoTable struct {
	ti       *TableInfo
	partials []*ast.TablePartial
	enums    []string
	diags    []diag.Diagnostic
}

func (m *Memo) lookup(d *ast.Table, partials []*ast.TablePartial, enums []string) *memoTable {
	if m == nil {
		return nil
	}
	e := m.prev[d]
	if e == nil || !slices.Equal(e.enums, enums) || len(e.partials) != len(partials) {
		return nil
	}
	for i, p := range partials {
		if e.partials[i] != p {
			return nil
		}
	}
	return e
}

func (m *Memo) store(e *memoTable) {
	if m.next == nil {
		m.next = map[*ast.Table]*memoTable{}
	}
	m.next[e.ti.Decl] = e
}

func (m *Memo) finish() {
	m.prev, m.next = m.next, nil
}
