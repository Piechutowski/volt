// Selects of every shape (spec §selects-over-groups): scalar parameters
// over the model row, a list parameter, literals, a shared projection, a
// star-form exclusion, an ordering and a bare select, so volt_selects.go
// has a golden beside the other generated files.
package metrics

table page_views {
  id    int [pk]
  site  string
  day   int
  hits  int [default: 0]
  score float64 [default: 0]
  note  string [null]
}

table link_clicks {
  id     int [pk]
  site   string
  day    int
  target string [default: ""]
}

group traffic {
  page_views
  link_clicks
}

pred at    { site = :site and day = :day }
pred since { day >= :from }

select rows    for page_views where at
select chosen  for traffic where site in :sites
select busy    for page_views where hits > 10 and score >= 0.5 and note is not null
select summary (site, day) for traffic where since
select public  (* \ target) for link_clicks
select recent  for traffic where since [order: (day desc, id asc)]
select all     for link_clicks

// Joins (spec §selects-over-groups, §relationships-ref): a forward
// relation joins inner, a nullable one left with its columns rt.Null, and
// a reverse relation, named outright where two would collide, joins left.
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

select hosted  (id, day, host.name) for visits join host where day >= :from [order: (host.name asc, id desc)]
select owned   (id, host.name, owner.name) for visits join host join owner
select traffic (id, name, visits.day) for hosts join visits where visits.day = :day

// Aggregates, arithmetic and group: (spec §selects-over-groups): a
// grouped select keeps one row per group, an ungrouped one yields one
// row as a lookup with its aggregates rt.Null, and an expression binds
// its numbers as arguments.
select daily  (site, day / 7 * 7 as week, sum(hits) as total, avg(score) as mean, count() as n) for page_views where day >= :from [group: (site, week), order: (site asc, week asc)]
select totals (count() as n, max(hits) as top, min(note) as first) for page_views
select scaled (id, hits * :factor + 1 as scaled) for page_views

// Nested results (spec §selects-over-groups): each host with its visits,
// the children loaded by one statement over the parent keys.
select board (* \ name) for hosts with visits where id >= :from [order: (id asc)]
select lineup for hosts with visits

// A composite key joins on every pair, the forward relation named after
// the referenced table (spec §relationships-ref).
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

select placed (id, merchants.name) for merchant_periods join merchants where merchants.name like :pattern

// Limits (spec §selects-over-groups): LIMIT ? after ORDER BY, a literal
// bound as an argument, a parameter last in the signature and checked by
// the effect, on every member of a group, after a grouping, and on the
// parents of a nesting.
select top     for page_views where day >= :from [order: (hits desc, id asc), limit: :limit]
select first   for traffic [order: (day asc, id asc), limit: 3]
select leading (site, sum(hits) as total) for page_views [group: (site), order: (total desc, site asc), limit: :n]
select newest  for hosts with visits [order: (id desc), limit: :limit]

// Optional parameters and exists (spec §predicates): a term that reads an
// absent optional parameter is true behind its guard, an optional list
// binds through JSONListOptional, and an exists term tests a forward or a
// reverse relation without joining it, a composite key on every pair.
select keyset  for visits where day < :day? or (day = :day? and id < :id?) [order: (day desc, id desc), limit: :limit]
select picked  for page_views where id in :ids? and site like :site? [order: (id asc)]
select visited for hosts where exists visits (day >= :from?) and not exists owned (day = 0) [order: (id asc)]
select by_host (id, day) for visits where exists host (name = :name)
select active  for merchants where exists merchant_periods (id > 0)
