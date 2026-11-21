module github.com/Piechutowski/volt/cmd/volt

go 1.27

require (
	github.com/Piechutowski/volt v0.0.0
	github.com/Piechutowski/volt/lsp v0.0.0
	github.com/mattn/go-sqlite3 v1.14.47
	github.com/urfave/cli/v3 v3.10.1
	golang.org/x/tools v0.50.0
)

require (
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
)

// Load-bearing until this tree lands on main and gets v-tags: without these
// `go build` here resolves the modules from main, which still has the
// pre-split flat lang/ and no lsp module at all. Do not delete.
replace (
	github.com/Piechutowski/volt => ../..
	github.com/Piechutowski/volt/lsp => ../../lsp
)
