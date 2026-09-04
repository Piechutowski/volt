// The integration fixture: one schema exercising every v0 CRUD shape and
// the v2 dynamic query layer against a real SQLite (decision D25). The
// generated siblings (nao_models.go, nao_queries.go, nao_dyn.go,
// nao_schema.sql) are checked in and drift-tested; refresh them with
// 'volt gen ./nao/itest --sql' from the repository root.
package itest

Project itest {
  note: 'Integration-test fixture for the generated query surface.'
}

Enum order_status {
  pending
  shipped
  delivered [note: 'Terminal state']
}

Table users {
  id integer [pk, increment]
  email varchar [not null, unique]
  name varchar [not null, tag: 'json:"displayName"']
  bio text [note: 'NULL until the user writes one']
  created_at timestamp [not null, default: `CURRENT_TIMESTAMP`]

  checks {
    EmailValid(email)
  }
}

Table orders {
  id integer [pk, increment]
  user_id integer [not null, ref: > users.id]
  status order_status [not null, default: order_status.pending]
  total decimal(10,2) [not null, required]
  placed_at timestamp
}

// composite primary key, no auto-increment anywhere
Table user_tags {
  user_id integer [not null, ref: > users.id]
  tag varchar [not null]

  indexes {
    (user_id, tag) [pk]
  }
}

// Two uniform metric tables plus the group/pred/select layer over them
// (spec §V9-§V11): one select, one signature, every member.
Table page_views {
  id integer [pk, increment]
  site varchar [not null]
  day integer [not null]
  hits integer [not null, default: 0]

  checks {
    hits >= 0 and day >= 1 [name: 'counts_positive']
    site like '%_'
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

Pred at { site = :site and day = :day }
Pred since { day >= :from }

Select rows for metrics where at [order: (day desc, id asc)]
Select recent for metrics where at or since

// §V10.3 list parameter: one JSON array, unpacked by json_each (D66).
Select on_sites for metrics where site in :sites [order: (id asc)]

// §V11.7 projections: an explicit list mints one shared row type for
// every member; the star form mints a per-member struct derivative.
Select summary (site, day) for metrics where at
Select public (* \ target) for link_clicks [order: (id asc)]
