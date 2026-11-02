// spec: §predicates, §selects-over-groups — `in :name` is a list parameter typed as a slice of the column's type
package db

table page_views {
  id int [pk]
  site string
  day int
  seen bool [default: false]
}

pred chosen { site in :sites and day in :days }

select picked for page_views where chosen and seen in :flags [order: (id asc)]
