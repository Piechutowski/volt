package db

table metrics {
  id int       [pk]
  at time.Time
}

select since for metrics where at > :since
