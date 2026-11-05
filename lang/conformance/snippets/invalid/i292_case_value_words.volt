// spec: §keywords, §selects-over-groups, §the-action-table, an order direction and a resource action are lowercase keywords
// want: write desc, not DESC
package db

table posts {
  id    int [pk]
  views int
}

select top for posts [order: (views DESC)]

scope /api {
	resources posts [only: (Index, show)]
}
