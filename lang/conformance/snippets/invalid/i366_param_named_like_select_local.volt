// spec: §predicates, a parameter name is none of the names the generated select signatures use and does not take the form of the generated locals
// want: parameter name "q" does not work as a Go parameter
package app

Table users {
	id   int    [pk]
	nick string [null]
}

Select by_q        for users where nick = :q
Select by_rt       for users where nick = :rt
Select by_rows     for users where nick = :voltRows
Select by_err      for users where nick in :voltErr
