// spec: §V11.7 — explicit list mints one shared row type; star-minus
// mints per-member struct derivatives; where/order columns need not be
// projected
package db

Table page_views {
  id integer [pk, increment]
  site varchar [not null]
  day integer [not null]
  hits integer [not null, default: 0]
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

Select summary (site, day) for metrics where day >= :from
Select public (* \ site) for metrics [order: (id asc)]
