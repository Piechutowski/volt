// spec: §predicates, a parameter name is none of the names the generated select method uses
// want: parameter name "rows" does not work as a Go parameter
package app

Table users {
	id   int    [pk]
	nick string [null]
}

Select by_nick for users where nick = :rows
Select by_err  for users where nick = :err
Select by_out  for users where nick in :out
Select by_rt   for users where nick = :rt
