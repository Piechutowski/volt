// spec: §settings-lists, §selects-over-groups, a select takes one limit:, a second is the repeated setting every settings list refuses
// want: setting "limit" may appear at most once
package db

table page_views {
  id   int [pk]
  hits int
}

select top for page_views [order: (hits desc, id asc), limit: 10, limit: :limit]
