package db

Table metrics {
  id   int    [pk, increment]
  slug string [unique]
  unit string
}

Table settings {
  id      int [pk]
  refresh int [default: 60]
}

Select ordered for metrics [order: (slug asc)]
Select by_unit for metrics where unit = :unit and id in :ids
