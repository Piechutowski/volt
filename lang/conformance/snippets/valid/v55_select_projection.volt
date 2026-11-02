// spec: §selects-over-groups — explicit list mints one shared row type; star-minus
// mints per-member struct derivatives; where/order columns need not be
// projected
package db

table page_views {
  id int [pk, increment]
  site string
  day int
  hits int [default: 0]
}

table link_clicks {
  id int [pk, increment]
  site string
  day int
  target string [default: ""]
}

group metrics {
  page_views
  link_clicks
}

select summary (site, day) for metrics where day >= :from
select public (* \ site) for metrics [order: (id asc)]
