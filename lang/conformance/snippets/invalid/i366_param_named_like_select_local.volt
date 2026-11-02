// spec: §predicates, a parameter name is none of the names the generated select signatures use and does not take the form of the generated locals
// want: parameter name "q" does not work as a Go parameter
package app

table users {
	id   int    [pk]
	nick string [null]
}

select by_q        for users where nick = :q
select by_rt       for users where nick = :rt
select by_rows     for users where nick = :voltRows
select by_err      for users where nick in :voltErr
