// The integration fixture: one schema exercising every v0 CRUD shape and
// the v2 dynamic query layer against a real SQLite. The
// generated siblings (nao_models.go, nao_queries.go, nao_dyn.go,
// nao_schema.sql) are checked in and drift-tested; refresh them with
// 'volt gen ./nao/itest --sql' from the repository root.
package itest

Type Money int
Type pay.Cents int from 'github.com/Piechutowski/volt/nao/itest/pay'

Project itest [note: 'Integration-test fixture for the generated query surface.'] {
}

Type OrderState string
Type Featured bool
Type Ratio float64

Table users {
  id int [pk, increment]
  email string [unique]
  name string [tag: 'json:"displayName"']
  bio string [null, note: 'NULL until the user writes one']
  created_at time.Time [default: `CURRENT_TIMESTAMP`]

  validators {
    EmailValid(email)
  }
}

Table orders {
  id int [pk, increment]
  user_id int [ref: users.id]
  status OrderState [default: 'pending']
  total float64 [required]
  tip Money [default: 0]
  fee pay.Cents
  placed_at time.Time [null]

  validators {
    OrderStateKnown(status)
  }
}

// §types rule 4, §predicates rule 5: a package-qualified declared type
// compares with = and in, and a literal has the class of its storage.
Select by_fee for orders where fee = :fee
Select fees   for orders where fee in :fees [order: (id asc)]
Select free   for orders where fee = 0

// §types rule 3, §predicates rule 5: a declared type whose Go declaration
// spells its storage type binds by kind, a list of it as one JSON array.
Select by_state for orders where status in :states [order: (id asc)]

// composite primary key, no auto-increment anywhere
Table user_tags {
  user_id int [ref: users.id]
  tag string

  indexes {
    (user_id, tag) [pk]
  }
}

// Two uniform metric tables plus the group/pred/select layer over them
// (spec §groups to §selects-over-groups): one select, one signature, every member.
Table page_views {
  id int [pk, increment]
  site string
  day int
  hits int [default: 0]

  validators {
    CountsPositive(hits, day)
    SiteShape(site)
  }
}

Table link_clicks {
  id int [pk, increment]
  site string
  day int
  target string [default: '']
}

Group metrics {
  page_views
  link_clicks
}

Pred at { site = :site and day = :day }
Pred since { day >= :from }

Select rows for metrics where at [order: (day desc, id asc)]
Select recent for metrics where at or since

// §predicates list parameter: one JSON array, unpacked by json_each.
Select on_sites for metrics where site in :sites [order: (id asc)]

// §selects-over-groups projections: an explicit list mints one shared row type for
// every member; the star form mints a per-member struct derivative.
Select summary (site, day) for metrics where at
Select public (* \ target) for link_clicks [order: (id asc)]

// Kinded bool and float64 columns, plain and nullable (spec §types).
Table site_flags {
  id       int      [pk, increment]
  featured Featured
  pinned   Featured [null]
  ratio    Ratio
  weight   Ratio    [null]
}

// Bytes (spec §columns rule 2, §nullability): a []byte column without
// null is bytes and nil writes empty bytes, one with null is
// rt.Null[[]byte], and required is the one refusal of empty bytes.
Table attachments {
  id     int    [pk, increment]
  name   string [unique]
  body   []byte
  thumb  []byte [null]
  digest []byte [required]
}

Select named for attachments where name = :name

// A []byte key binds as every plain []byte does, on every key path.
Table blobs {
  hash []byte [pk]
  tag  []byte [unique]
  size int    [default: 0]
}
