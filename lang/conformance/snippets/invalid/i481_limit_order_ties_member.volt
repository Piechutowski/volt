// spec: §selects-over-groups, over a group a limit's order names a key of every member
// want: order: names no key of link_clicks
package db

table page_views {
  id  int [pk]
  day int
}

table link_clicks {
  ref string [pk]
  id  int
  day int
}

group traffic {
  page_views
  link_clicks
}

select first for traffic [order: (day asc, id asc), limit: 5]
