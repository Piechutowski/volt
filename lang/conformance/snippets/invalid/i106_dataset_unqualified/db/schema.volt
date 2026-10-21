// spec: §groups, §selects-over-groups — the group select a dataset expands
package db

Table ms_revenue {
  id int [pk, increment]
  org string
  year int
}

Table ms_usage {
  id int [pk, increment]
  org string
  year int
}

Table ms_notes {
  id int [pk, increment]
  org string
  year int
}

Group series {
  ms_revenue
  ms_usage
  ms_notes
}

Select browse for series where year = :year and org in :orgs [order: (id asc)]
