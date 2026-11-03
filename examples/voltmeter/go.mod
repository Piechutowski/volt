module github.com/Piechutowski/volt/examples/voltmeter

go 1.27

require (
	github.com/Piechutowski/volt v0.0.0
	github.com/mattn/go-sqlite3 v1.14.47
)

// Load-bearing until this tree lands on main and gets v-tags, as in
// cmd/volt/go.mod: without it `go build` here resolves the root module
// from main. Do not delete.
replace github.com/Piechutowski/volt => ../..
