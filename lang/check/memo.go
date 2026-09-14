package check

// The per-table memo (D84, PERF-10): a table whose declaration node and
// injected partials' nodes are the objects they were at the last check
// of this file is the same checked table with the same diagnostics.
// Node identity is the key because the parser reuses a declaration's
// nodes exactly when its text is unchanged (D83), and the memo holds
// the nodes it keys on, so an address is never reused under it. Every
// entry not hit by a check is dropped after it.

import (
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
	ti         *TableInfo
	partials   []*ast.TablePartial // the injected partials, in body order, nil for a missing one
	hasImports bool
	diags      []diag.Diagnostic // the table's own: expansion, body, columns
}

// lookup answers the table for d when its inputs are what they were.
func (m *Memo) lookup(d *ast.Table, partials []*ast.TablePartial, hasImports bool) *memoTable {
	e := m.prev[d]
	if e == nil || e.hasImports != hasImports || len(e.partials) != len(partials) {
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

// finish drops what the last check did not use.
func (m *Memo) finish() {
	m.prev, m.next = m.next, nil
}
