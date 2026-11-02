// Prepare-validation fixture (spec §selects-over-groups): every select the
// generator would emit for this package is prepared against the DDL
// gen/sqlite emits from the same schema.
package db

table page_views {
  id int [pk]
  site string
  day int
  hits int [default: 0]
}

table link_clicks {
  id int [pk]
  site string
  day int
  target string [default: ""]
}

group metrics {
  page_views
  link_clicks
}

pred at    { site = :site and day = :day }
pred since { day >= :from }

select rows    for metrics where at or since [order: (day desc, id asc)]
select summary (site, day) for metrics where at
select public  (* \ target) for link_clicks
select named   for page_views where site like "%a%" and hits in (1, 2) and not (day < 1)
select chosen  for metrics where site in :sites and day in :days

table hosts {
  id   int [pk]
  name string
}

table visits {
  id       int [pk]
  host_id  int [ref: hosts.id]
  owner_id int [null]
  day      int
}

ref: visits.owner_id > hosts.id [as: owner, reverse: owned]

select hosted  (id, day, host.name) for visits join host where day >= :from and host.name like "%a%" [order: (host.name asc, id desc)]
select owned   (id, host.name, owner.name) for visits join host join owner where owner.name is null
select traffic (id, name, visits.day) for hosts join visits where visits.day in :days

select daily  (site, day / 7 * 7 as week, sum(hits) as total, count() as n) for page_views where day >= :from [group: (site, week), order: (site asc, week asc)]
select totals (count() as n, max(hits) as top) for page_views
select scaled (id, hits * :factor + 1 as scaled) for page_views
select hosted_hits (host.name as host, count(id) as visits, max(day) as last) for visits join host [group: (host)]

select board  for hosts with visits where name like "%a%" [order: (id asc)]
select lineup (id, name) for hosts with owned

table merchants {
  id           int    [pk]
  country_code string [pk]
  name         string
}

table merchant_periods {
  id           int    [pk]
  merchant_id  int
  country_code string
}

ref: merchant_periods.(merchant_id, country_code) > merchants.(id, country_code)

select placed (id, merchants.name) for merchant_periods join merchants
