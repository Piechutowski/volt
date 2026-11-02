// spec: §groups, §selects-over-groups — the group select a dataset expands
package db

table ms_revenue {
  id int [pk]
  org string
  year int
}

table ms_usage {
  id int [pk]
  org string
  year int
}

table ms_notes {
  id int [pk]
  org string
  year int
}

group series {
  ms_revenue
  ms_usage
  ms_notes
}

select browse for series where year = :year and org in :orgs [order: (id asc)]
