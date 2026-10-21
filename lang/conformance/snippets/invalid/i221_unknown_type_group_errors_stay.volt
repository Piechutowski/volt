// spec: §columns, §selects-over-groups (an unknown type silences only what depends on it)
// want: is missing from link_clicks
package db

Enum mood {
  happy
  sad
}

Table page_views {
  id   int    [pk, increment]
  hits int32
}

Table link_clicks {
  id   int    [pk, increment]
  path string
}

Table feed_reads {
  id   int    [pk, increment]
  hits int
}

Table search_hits {
  id   int    [pk, increment]
  hits string
}

Table mood_marks {
  id   int  [pk, increment]
  hits mood
}

Table counteds {
  id int [pk, increment]
}

Group lacking {
  page_views
  link_clicks
}

Group split {
  page_views
  feed_reads
  search_hits
}

Group moody {
  page_views
  mood_marks
}

Select busy for lacking where hits > :min
Select mixed for split where hits = :h
Select glum for moody where hits = :h
Select tally (hits) for lacking
Select spread (hits) for split
Select ranked for page_views where hits > :min [order: (hits desc)]
Select counted (hits) for page_views
Select located (hits) for page_views where nope = :x
Select hits (* \ id) for page_views
Select page_view_hits (hits) for page_views
