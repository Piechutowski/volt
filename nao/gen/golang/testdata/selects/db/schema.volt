// Prepare-validation fixture (spec §selects-over-groups): every select the
// generator would emit for this package is prepared against the DDL
// gen/sqlite emits from the same schema.
package db

Table page_views {
  id int [pk, increment]
  site string
  day int
  hits int [default: 0]
}

Table link_clicks {
  id int [pk, increment]
  site string
  day int
  target string [default: ""]
}

Group metrics {
  page_views
  link_clicks
}

Pred at    { site = :site and day = :day }
Pred since { day >= :from }

Select rows    for metrics where at or since [order: (day desc, id asc)]
Select summary (site, day) for metrics where at
Select public  (* \ target) for link_clicks
Select named   for page_views where site like "%a%" and hits in (1, 2) and not (day < 1)
Select chosen  for metrics where site in :sites and day in :days
