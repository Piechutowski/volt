package check

// The per-table memo (decision check-by-declaration): a table whose declaration node and
// injected partials' nodes are the objects they were at the last check
// of this file is the same checked table with the same diagnostics.
// Node identity is the key (decision parse-by-declaration); the memo holds the nodes it keys on,
// so an address is never reused under it.

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

// memoTable is one call of tableCheck: its inputs and its outputs.
type memoTable struct {
	ti         *TableInfo
	partials   []*ast.TablePartial // the injected partials, in body order, nil for a missing one
	hasImports bool
	enums      []string // the enum set, sorted: the keys themselves, not a spelling of them (decision keys-are-inputs)
	diags      []diag.Diagnostic
}

// lookup answers the table for d when every input of tableCheck is
// what it was; a nil memo answers nothing.
func (m *Memo) lookup(d *ast.Table, partials []*ast.TablePartial, hasImports bool, enums []string) *memoTable {
	if m == nil {
		return nil
	}
	e := m.prev[d]
	if e == nil || e.hasImports != hasImports || !slices.Equal(e.enums, enums) || len(e.partials) != len(partials) {
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
