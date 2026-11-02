// spec: §selects-over-groups — an alias names no column of a joined table, which SQLite would group by
// want: names a column of "metrics", joined as "metric"
package db

Table metrics {
  id   int    [pk, increment]
  name string
  unit string [null]
  hour int
}

Table points {
  id        int   [pk, increment]
  metric_id int   [ref: metrics.id]
  ts        int
  value     float64
}

Select hourly (metric_id, ts / 3600 as hour, count() as n) for points join metric [group: (metric_id, hour)]
