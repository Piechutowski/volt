package naming

import "github.com/Piechutowski/volt/nao/rt"

// SelectParam is one Go parameter of a generated select method
// (spec §selects-over-groups), in declaration order.
type SelectParam struct {
	GoName string
	GoType string // e.g. "string", "int64", "time.Time"
}

// SelectFn is one select instantiation: a query function for one
// member table, over the predicate tree and order terms the checker
// lowered (spec §predicates) and the §selects-over-groups
// projection already validated.
type SelectFn struct {
	TableKey     string // the table's name, its key into check.Info
	MethodSuffix string
	Where        rt.GenPred    // the zero value when the select has no where clause
	Order        []rt.GenOrder // nil when it has no order: setting
	Params       []SelectParam
	Items        []SelectItem // explicit projection, declared order; nil = all or star form
	Excluded     []string     // star-form exclusions; nil = none
	SharedType   string       // explicit list: the one shared row type name
	Joins        []SelectJoin
	Group        []string // the group: items as the statement names them, a column or an alias; nil = none
	Lookup       bool     // the select aggregates without group:, so it yields one row (spec §selects-over-groups)
	With         *SelectWith
}

// SelectWith is the nesting of a select (spec §selects-over-groups): the
// reverse relation whose rows each parent carries, its table, the key
// columns the rows match on, the parent's and the child's, and the Go
// names of the field that holds the children and of the row type.
type SelectWith struct {
	Name      string
	TableKey  string
	ParentKey string
	ChildKey  string
	Field     string
	RowType   string
}

// SelectItem is one projected item (spec §selects-over-groups): a column,
// named as the statement names it, a joined one as relation.column; or a
// derived item, an aggregate over a column (or the rows, for count with
// no column) or an expression, under its alias and with its Go type,
// rt.Null wrapped where it may be NULL.
type SelectItem struct {
	Col    string
	Agg    string
	Expr   rt.Expr
	Alias  string
	GoType string
}

// Derived reports whether the item is an aggregate or an expression.
func (it SelectItem) Derived() bool { return it.Alias != "" }

// Aggregate reports whether the item is an aggregate.
func (it SelectItem) Aggregate() bool { return it.Agg != "" }

// SelectJoin is one join of a select (spec §selects-over-groups): the
// relation's name, its table, whether it joins LEFT, and the column
// pairs the join matches, the joined table's column first.
type SelectJoin struct {
	Name     string
	TableKey string
	Left     bool
	On       [][2]string
}

// FieldSig is one generated struct field.
type FieldSig struct {
	Name     string // Go field name, e.g. "EditorID"
	Col      string // source column name, e.g. "editor_id"
	Type     string // full Go type, e.g. "rt.Null[int64]"
	Tag      string // assembled struct tag (§struct-tags)
	Doc      string // column note, "" when absent
	Nullable bool   // NULL-admitting column (§nullability)
}

// ValidatorFn is one table's validator surface (spec §validators): what
// its changeset's Validate runs after required.
type ValidatorFn struct {
	TableKey   string
	Validators []ValidatorSpec
}

// ValidatorSpec is one lowered validator, resolved by the Volt checker
// (spec §validators): the generated Validate calls Name with the row's
// Fields in order.
type ValidatorSpec struct {
	Name   string   // the validator's function name, the check of its field error
	Src    string   // human-readable form, e.g. "EmailValid(email)"
	Fields []string // the Go fields of the row it passes, in order, e.g. "Email"
	Cols   []string // the columns the validator reads, in first-appearance order
}

// CRUDMethod is one generated default-CRUD method of a table, as a query
// route (spec §query-routes) sees it: the name to call, the identity
// parameters in key order, the changeset a write takes and how the route
// builds it (spec §changesets), and what comes back.
type CRUDMethod struct {
	Name   string        // method name, e.g. "UserGet"
	Op     string        // get | list | create | update | delete
	Key    []SelectParam // identity parameters (get, update, delete); nil otherwise
	Body   string        // changeset type (create, update), e.g. "UserChangeset"; "" otherwise
	Params string        // write body type the route decodes, e.g. "UserParams"; "" otherwise
	New    string        // constructor a create starts from, e.g. "UserNew"; "" otherwise
	Get    string        // loader an update reads the row with, e.g. "UserGet"; "" otherwise
	Allow  []string      // column handles the route's Cast allows, e.g. "UserEmail"; nil otherwise
	Result string        // row type; "" for delete
	Many   bool          // slice result (list)
}
