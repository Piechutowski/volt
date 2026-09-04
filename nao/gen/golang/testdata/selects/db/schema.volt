// Prepare-validation fixture (spec §V11.6, D06): every select the
// generator would emit for this package is prepared against the DDL
// gen/sqlite emits from the same schema.
package db

Table page_views {
  id integer [pk, increment]
  site varchar [not null]
  day integer [not null]
  hits integer [not null, default: 0]

  checks {
    hits >= 0 [name: 'hits_positive']
  }
}

Table link_clicks {
  id integer [pk, increment]
  site varchar [not null]
  day integer [not null]
  target text [not null, default: '']
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
Select named   for page_views where site like '%a%' and hits in (1, 2) and not (day < 1)
Select chosen  for metrics where site in :sites and day in :days
