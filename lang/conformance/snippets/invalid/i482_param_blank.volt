// spec: §predicates, a parameter is never the blank identifier, which no generated expression can read
// want: parameter name "_" does not work as a Go parameter
package db

table page_views {
  id   int [pk]
  hits int
}

select busy for page_views where hits > :_
