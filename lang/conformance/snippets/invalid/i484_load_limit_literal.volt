// spec: §route-loads, §selects-over-groups, a literal a load binds to a select's limit is at least 1, or every request would be refused
// want: is its limit, at least 1, not 0
package app

table posts {
  id   int [pk]
  hits int
}

select top for posts [order: (hits desc, id asc), limit: :limit]

scope / {
  get /home Pages.Home {
    none PostTop(0)
    less PostTop(-1)
  }
}
