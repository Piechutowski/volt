// spec: §groups, §selects-over-groups, a group is a set: a group term, a select target, subtracted as a parenthesized set
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

table ms_dict {
  code string [pk]
  title string
}

table ms_notes {
  id int [pk]
  body string
}

group Metrics {
  ms_revenue
  ms_usage
  ms_dict
  ms_notes
}

group series = Metrics \ (ms_dict, ms_notes)
group with_dict = series + (ms_dict)

pred current { org = :org and year = :year }

select rows for series where current [order: (year desc, id asc)]
select everything for Metrics
