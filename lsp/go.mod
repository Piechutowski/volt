module github.com/Piechutowski/volt/lsp

go 1.27

require github.com/Piechutowski/volt v0.0.0

// Load-bearing until this tree lands on main and gets a v-tag: without it
// `go build` here resolves github.com/Piechutowski/volt@latest from main,
// which still has the pre-split flat lang/ and no lang/ast, lang/parser, ...
// Do not delete. go.work covers workspace builds; this covers `cd lsp`.
replace github.com/Piechutowski/volt => ..
