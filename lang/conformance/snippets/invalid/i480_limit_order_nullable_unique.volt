// spec: §selects-over-groups, a unique column that may be NULL is no key for a limit's order, since NULLs repeat under unique
// want: takes limit: over an order that leaves ties: order: names no key of posts
package db

table posts {
  id   int    [pk]
  slug string [unique, null]
}

select by_slug for posts [order: (slug asc), limit: 10]
