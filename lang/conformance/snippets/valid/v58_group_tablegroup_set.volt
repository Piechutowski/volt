// spec: §V9.2, §V9.3, §V11.2 — a TableGroup is a set: a group term, a select target, subtracted as a parenthesized set (D65)
package db

Table ms_revenue {
  id integer [pk]
  org text [not null]
  year integer [not null]
}

Table ms_usage {
  id integer [pk]
  org text [not null]
  year integer [not null]
}

Table ms_dict {
  code text [pk]
  title text [not null]
}

Table ms_notes {
  id integer [pk]
  body text [not null]
}

TableGroup Metrics [color: #32a891] {
  ms_revenue
  ms_usage
  ms_dict
  ms_notes
}

Group series = Metrics \ (ms_dict, ms_notes)
Group with_dict = series + (ms_dict)

Pred current { org = :org and year = :year }

Select rows for series where current [order: (year desc, id asc)]
Select everything for Metrics
