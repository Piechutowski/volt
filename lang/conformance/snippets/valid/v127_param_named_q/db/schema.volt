// spec: §predicates, §generated-query-surface, q names a select parameter, the usual search term, and a key column's parameter, since no generated function uses it
package db

table posts {
  id    int    [pk]
  title string
}

table words {
  q     string [pk]
  uses  int
}

select search for posts where title like :q? [order: (id asc)]
