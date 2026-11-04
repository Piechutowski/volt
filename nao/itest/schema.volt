// The integration fixture: one schema exercising every v0 CRUD shape and
// the v2 dynamic query layer against a real SQLite. The
// generated siblings (volt_models.go, volt_queries.go, volt_dyn.go,
// volt_schema.sql) are checked in and drift-tested; refresh them with
// 'volt gen ./nao/itest --sql' from the repository root.
package itest

type Money int
type pay.Cents int from "github.com/Piechutowski/volt/nao/itest/pay"
type pay.YesNo string from "github.com/Piechutowski/volt/nao/itest/pay"

type OrderState string
type Featured bool
type Ratio float64

table users {
  id int [pk]
  email string [unique]
  name string [tag: "json:\"displayName\""]
  bio string [null, note: "NULL until the user writes one"]
  created_at time.Time [default: `CURRENT_TIMESTAMP`]

  validators {
    EmailValid(email)
  }
}

table orders {
  id int [pk]
  user_id int [ref: users.id]
  status OrderState [default: "pending"]
  total float64 [required]
  tip Money [default: 0]
  fee pay.Cents
  placed_at time.Time [null]

  validators {
    OrderStateKnown(status)
  }
}

// §types, §predicates: a package-qualified declared type
// compares with = and in, and a literal has the class of its storage.
select by_fee for orders where fee = :fee
select fees   for orders where fee in :fees [order: (id asc)]
select free   for orders where fee = 0

// §types, §predicates: a declared type whose Go declaration
// spells its storage type binds by kind, a list of it as one JSON array.
select by_state for orders where status in :states [order: (id asc)]

// composite primary key, no rowid alias
table user_tags {
  user_id int [pk, ref: users.id]
  tag string [pk]
}

// Two uniform metric tables plus the group/pred/select layer over them
// (spec §groups to §selects-over-groups): one select, one signature, every member.
table page_views {
  id int [pk]
  site string
  day int
  hits int [default: 0]

  validators {
    CountsPositive(hits, day)
    SiteShape(site)
  }
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

pred at { site = :site and day = :day }
pred since { day >= :from }

select rows for metrics where at [order: (day desc, id asc)]
select recent for metrics where at or since

// §predicates list parameter: one JSON array, unpacked by json_each.
select on_sites for metrics where site in :sites [order: (id asc)]

// §selects-over-groups projections: an explicit list mints one shared row type for
// every member; the star form mints a per-member struct derivative.
select summary (site, day) for metrics where at
select public (* \ target) for link_clicks [order: (id asc)]

// Kinded bool and float64 columns, plain and nullable (spec §types).
table site_flags {
  id       int      [pk]
  featured Featured
  pinned   Featured [null]
  ratio    Ratio
  weight   Ratio    [null]
}

// Bytes (spec §columns, §nullability): a []byte column without
// null is bytes and nil writes empty bytes, one with null is
// rt.Null[[]byte], and required is the one refusal of empty bytes.
table attachments {
  id     int    [pk]
  name   string [unique]
  body   []byte
  thumb  []byte [null]
  digest []byte [required]
}

select named for attachments where name = :name

// A projected row type and a struct derivative scan a []byte as the
// model does: empty bytes read back non-nil (spec §nullability).
select slim (id, body, thumb) for attachments where name = :name
select heavy (* \ name) for attachments where name = :name

// A []byte key binds as every plain []byte does, on every key path.
table blobs {
  hash []byte [pk]
  tag  []byte [unique]
  size int    [default: 0]
}

// A composite primary key is its pk columns in declaration order, apart
// or not (spec §columns): Get, Update and Delete take day, then site. A
// column's index is its own CREATE INDEX, unique or not (§indexes).
table daily_hits {
  day   int    [pk]
  hits  int    [default: 0]
  site  string [pk]
  label string [index, default: ""]
  token string [null, unique]
}

// A foreign key referencing a unique column, whose uniqueness is its
// CREATE UNIQUE INDEX (spec §relationships-ref, §indexes).
table invites {
  id         int    [pk]
  user_email string [ref: users.email]
  code       string [unique]
}

// A package-qualified type of Go kind bool that stores text: its own Scan
// and Value decide, plain and nullable (spec §types).
table post_flags {
  id       int       [pk]
  reviewed pay.YesNo
  approved pay.YesNo [null]
}

// Joins (spec §selects-over-groups): orders reach their user through the
// forward relation of user_id, inner since the key is required; users
// reach their orders through the reverse relation, left since a user may
// have none, so the joined columns are rt.Null.
select placed (id, total, user.name, user.email) for orders join user where total >= :min [order: (id asc)]
select buyers (id, name, orders.total) for users join orders [order: (id asc, orders.id asc)]

// Aggregates (spec §selects-over-groups): one row per group, and one row
// for the whole table when nothing groups.
select hourly (site, day / 2 * 2 as bucket, sum(hits) as total, avg(hits) as mean, count() as n) for page_views [group: (site, bucket), order: (site asc, bucket asc)]
select totals (count() as n, max(hits) as top, sum(hits) as total) for page_views

// min and max of a NOT NULL date/time column keep time.Time under group:,
// read from the text SQLite answers (spec §go-types).
select joined (name, min(created_at) as first, max(created_at) as last) for users [group: (name), order: (name asc)]

// Nested results (spec §selects-over-groups): a user with the orders that
// refer to it, two statements in one transaction.
select carts    for users with orders where id = :id
select everyone for users with orders [order: (id asc)]

// Limits (spec §selects-over-groups): the first rows in order, a literal
// on every member of the group, a parameter the effect checks before any
// statement runs, groups counted after the grouping, and parents counted
// under a nesting, each keeping all its children.
select first_days for metrics [order: (day asc, id asc), limit: 2]
select top_hits   for page_views where site = :site [order: (hits desc, id asc), limit: :limit]
select top_sites  (site, sum(hits) as total) for page_views [group: (site), order: (total desc, site asc), limit: :limit]
select newest     for users with orders [order: (id desc), limit: :limit]

// A date/time list parameter and a date/time with key travel as JSON
// arrays of the canonical text the column holds (spec §go-types).
select stamped for users where created_at in :stamps [order: (id asc)]

table batches {
  opened_at time.Time [pk]
  label     string    [default: ""]
}

table batch_items {
  id        int       [pk]
  opened_at time.Time [ref: batches.opened_at]
}

select trail for batches with batch_items [order: (opened_at asc)]

// §columns: the database-assigned key with SQLite's AUTOINCREMENT, whose
// ids are never reused, beside the plain database-assigned key of users.
table metric_events {
  id   int    [pk, autoincrement]
  name string
}

// §predicates: an optional parameter's term is true when it is absent, a
// keyset's first page among them, and exists tests a relation without
// joining it, so a grouped count beside it counts each row once.
select keyset     for page_views where day < :day? or (day = :day? and id < :id?) [order: (day desc, id desc), limit: :limit]
select picked     for page_views where site = :site? and hits >= :min? [order: (id asc)]
select in_sites   for page_views where site in :sites? [order: (id asc)]
select placed_by  for orders where placed_at < :before? [order: (id asc)]
select tagged     for users where exists user_tags (tag = :tag?) [order: (id asc)]
select bought_by  for orders where exists user (name = :name) [order: (id asc)]
select order_counts (id, count(orders.id) as n) for users join orders where exists user_tags (tag in :tags) [group: (id), order: (id asc)]
select joined_counts (id, count(orders.id) as n) for users join orders join user_tags where user_tags.tag in :tags [group: (id), order: (id asc)]
