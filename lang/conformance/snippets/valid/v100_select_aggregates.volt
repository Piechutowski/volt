// spec: §selects-over-groups — aggregates and arithmetic under aliases, grouped per group: or one row as a lookup
package db

table points {
  id        int     [pk, increment]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

select hourly (metric_id, ts / 3600 * 3600 as hour, avg(value) as mean, count() as n, max(note) as last) for points where ts >= :since [group: (metric_id, hour), order: (hour asc, mean desc)]
select totals (count() as n, sum(value) as total, min(ts) as first) for points
select scaled (id, value * :factor + 1 as scaled, (ts - :epoch) / 60 as minute) for points
select renamed (id, label as tag) for points
