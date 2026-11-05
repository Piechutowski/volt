// spec: §keywords, §predicates, a predicate keyword names no pred inside a predicate, so a bare one is no reference
// want: "null" is a keyword inside a predicate and names no pred there
package db

table posts {
  id    int    [pk]
  title string
}

select titled for posts where null
