package db

Table metrics {
  id   int    [pk, increment]
  slug string [unique]
  unit string
}

Table settings {
  id      int [pk]
  refresh int [default: 60]
  note    string
}

Select ordered for metrics [order: (slug asc)]
