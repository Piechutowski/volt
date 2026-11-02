// spec: §predicates, a parameter name is neither nil nor append, the predeclared names a generated select method uses
// want: parameter name "nil" does not work as a Go parameter
package app

table users {
	id   int    [pk]
	nick string [null]
	rank int
}

select by_nil    for users where nick = :nil
select by_append for users where rank in :append
