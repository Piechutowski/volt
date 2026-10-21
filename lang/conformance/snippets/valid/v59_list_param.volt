// spec: §predicates, §selects-over-groups — `in :name` is a list parameter typed as a slice of the column's type
package db

Table page_views {
  id int [pk]
  site string [not null]
  day int [not null]
  seen bool [not null, default: false]
}

Pred chosen { site in :sites and day in :days }

Select picked for page_views where chosen and seen in :flags [order: (id asc)]
