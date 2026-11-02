// Selects of every shape (spec §selects-over-groups): scalar parameters
// over the model row, a list parameter, literals, a shared projection, a
// star-form exclusion, an ordering and a bare select, so volt_selects.go
// has a golden beside the other generated files.
package metrics

Table page_views {
  id    int [pk, increment]
  site  string
  day   int
  hits  int [default: 0]
  score float64 [default: 0]
  note  string [null]
}

Table link_clicks {
  id     int [pk, increment]
  site   string
  day    int
  target string [default: ""]
}

Group traffic {
  page_views
  link_clicks
}

Pred at    { site = :site and day = :day }
Pred since { day >= :from }

Select rows    for page_views where at
Select chosen  for traffic where site in :sites
Select busy    for page_views where hits > 10 and score >= 0.5 and note is not null
Select summary (site, day) for traffic where since
Select public  (* \ target) for link_clicks
Select recent  for traffic where since [order: (day desc, id asc)]
Select all     for link_clicks

// Joins (spec §selects-over-groups, §relationships-ref): a forward
// relation joins inner, a nullable one left with its columns rt.Null, and
// a reverse relation, named outright where two would collide, joins left.
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

Select hosted  (id, day, host.name) for visits join host where day >= :from [order: (host.name asc, id desc)]
Select owned   (id, host.name, owner.name) for visits join host join owner
Select traffic (id, name, visits.day) for hosts join visits where visits.day = :day

// Aggregates, arithmetic and group: (spec §selects-over-groups): a
// grouped select keeps one row per group, an ungrouped one yields one
// row as a lookup with its aggregates rt.Null, and an expression binds
// its numbers as arguments.
Select daily  (site, day / 7 * 7 as week, sum(hits) as total, avg(score) as mean, count() as n) for page_views where day >= :from [group: (site, week), order: (site asc, week asc)]
Select totals (count() as n, max(hits) as top, min(note) as first) for page_views
Select scaled (id, hits * :factor + 1 as scaled) for page_views
