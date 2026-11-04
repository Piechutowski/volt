// spec: §selects-over-groups, §query-routes, limit: keeps the first rows in order: a literal and a parameter, over a table, over a group (each member's statement takes the limit), with group:, and with with (the limit counts the parents); a query route binds the limit parameter like any other
package db

table page_views {
  id   int    [pk]
  site string
  day  int
  hits int
}

table link_clicks {
  id   int    [pk]
  site string
  day  int
}

group traffic {
  page_views
  link_clicks
}

table metrics {
  id   int    [pk]
  name string
}

table points {
  id        int [pk]
  metric_id int [ref: metrics.id]
  ts        int
}

select top     for page_views where site = :site [order: (hits desc, id asc), limit: 10]
select page    for page_views where day >= :from [order: (day asc, id asc), limit: :limit]
select padded  for page_views [order: (id asc), limit: 007]
select first   for traffic [order: (day asc, id asc), limit: 5]
select recent  for traffic where day >= :from [limit: :limit, order: (day desc, id desc)]
select busiest (site, sum(hits) as total) for page_views [group: (site), order: (total desc, site asc), limit: :n]
select board   for metrics with points [order: (name asc), limit: :limit]

scope /api {
  get /pages/:from(int) PageViewPage
  get /recent           LinkClickRecent
  get /busiest          PageViewBusiest
  get /board            MetricBoard
}
