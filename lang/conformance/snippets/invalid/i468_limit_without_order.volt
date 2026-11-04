// spec: §selects-over-groups, limit: needs order:, or the rows it keeps would be arbitrary
// want: takes limit: without order:
package db

table page_views {
  id   int [pk]
  hits int
}

select top for page_views [limit: 10]
