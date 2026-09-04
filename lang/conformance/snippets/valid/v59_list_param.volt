// spec: §V10.3, §V11.6 — `in :name` is a list parameter typed as a slice of the column's type (D66)
package db

Table page_views {
  id integer [pk]
  site text [not null]
  day integer [not null]
  seen boolean [not null, default: false]
}

Pred chosen { site in :sites and day in :days }

Select picked for page_views where chosen and seen in :flags [order: (id asc)]
