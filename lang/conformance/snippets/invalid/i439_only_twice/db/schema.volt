package db

table metrics {
  id   int    [pk]
  slug string [unique]
  unit string
}

table settings {
  id      int [pk]
  refresh int [default: 60]
  note    string
}

select ordered for metrics [order: (slug asc)]
