// spec: §selects-over-groups — group: names items by their name or alias
// want: project metric.name under an alias
package db

table metrics {
  id   int [pk, increment]
  name string
}

table points {
  id        int     [pk, increment]
  metric_id int [ref: metrics.id]
  ts        int
  value     float64
  label     string
  note      string  [null]
}

select rows (metric.name, count() as n) for points join metric [group: (metric.name)]
