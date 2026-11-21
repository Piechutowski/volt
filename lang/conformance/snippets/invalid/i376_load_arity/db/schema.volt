package db

table metrics {
  id   int    [pk]
  slug string [unique]
  unit string
}

table settings {
  id      int [pk]
  refresh int [default: 60]
}

select ordered for metrics [order: (slug asc)]
select by_unit for metrics where unit = :unit and id in :ids
