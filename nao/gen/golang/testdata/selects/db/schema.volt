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

Table hosts {
  id   int [pk, increment]
  name string
}

Table visits {
  id       int [pk, increment]
  host_id  int [ref: hosts.id]
  owner_id int [null]
  day      int
}

Ref: visits.owner_id > hosts.id [as: owner, reverse: owned]

Select hosted  (id, day, host.name) for visits join host where day >= :from and host.name like "%a%" [order: (host.name asc, id desc)]
Select owned   (id, host.name, owner.name) for visits join host join owner where owner.name is null
Select traffic (id, name, visits.day) for hosts join visits where visits.day in :days

Select daily  (site, day / 7 * 7 as week, sum(hits) as total, count() as n) for page_views where day >= :from [group: (site, week), order: (site asc, week asc)]
Select totals (count() as n, max(hits) as top) for page_views
Select scaled (id, hits * :factor + 1 as scaled) for page_views
Select hosted_hits (host.name as host, count(id) as visits, max(day) as last) for visits join host [group: (host)]
