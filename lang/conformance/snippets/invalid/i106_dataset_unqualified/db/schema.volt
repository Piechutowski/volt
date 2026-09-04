// spec: §V9, §V11 — the group select a dataset expands
package db

Table ms_revenue {
  id integer [pk, increment]
  org text [not null]
  year integer [not null]
}

Table ms_usage {
  id integer [pk, increment]
  org text [not null]
  year integer [not null]
}

Table ms_notes {
  id integer [pk, increment]
  org text [not null]
  year integer [not null]
}

Group series {
  ms_revenue
  ms_usage
  ms_notes
}

Select browse for series where year = :year and org in :orgs [order: (id asc)]
