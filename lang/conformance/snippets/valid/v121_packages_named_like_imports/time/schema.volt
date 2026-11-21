package time

table ticks {
  id int       [pk]
  at time.Time
}

select since for ticks where at > :at [order: (id asc)]

scope /time [name: time] {
  get /since TickSince
}
