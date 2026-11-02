// spec: §columns, §selects-over-groups (an unknown type silences only what depends on it)
// want: is missing from link_clicks
package db

table page_views {
  id   int    [pk, increment]
  hits int32
}

table link_clicks {
  id   int    [pk, increment]
  path string
}

table feed_reads {
  id   int    [pk, increment]
  hits int
}

table search_hits {
  id   int    [pk, increment]
  hits string
}

table raw_marks {
  id   int    [pk, increment]
  hits []byte
}

table counteds {
  id int [pk, increment]
}

group lacking {
  page_views
  link_clicks
}

group split {
  page_views
  feed_reads
  search_hits
}

group rawish {
  page_views
  raw_marks
}

select busy for lacking where hits > :min
select mixed for split where hits = :h
select raw for rawish where hits = :h
select tally (hits) for lacking
select spread (hits) for split
select ranked for page_views where hits > :min [order: (hits desc)]
select counted (hits) for page_views
select located (hits) for page_views where nope = :x
select hits (* \ id) for page_views
select page_view_hits (hits) for page_views
