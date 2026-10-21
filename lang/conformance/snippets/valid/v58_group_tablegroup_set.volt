// spec: §groups, §groups, §selects-over-groups — a TableGroup is a set: a group term, a select target, subtracted as a parenthesized set
package db

Table ms_revenue {
  id int [pk]
  org string
  year int
}

Table ms_usage {
  id int [pk]
  org string
  year int
}

Table ms_dict {
  code string [pk]
  title string
}

Table ms_notes {
  id int [pk]
  body string
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
