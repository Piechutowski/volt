// spec: §predicates, a parameter name is neither nil nor append, the predeclared names a generated select method uses
// want: parameter name "nil" does not work as a Go parameter
package app

Table users {
	id   int    [pk]
	nick string [null]
	rank int
}

Select by_nil    for users where nick = :nil
Select by_append for users where rank in :append
