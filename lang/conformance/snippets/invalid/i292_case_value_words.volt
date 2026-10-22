// spec: §keywords, §selects-over-groups, §the-action-table, an order direction and a resource action are lowercase keywords
// want: write desc, not DESC
package db

Table posts {
  id    int [pk, increment]
  views int
}

Select top for posts [order: (views DESC)]

Scope /api {
	resources posts [only: (Index, show)]
}
