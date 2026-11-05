// spec: §selects-over-groups — group: lists the items that are not aggregates
// want: names the aggregate
package db

table points {
  id        int     [pk]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

select rows (metric_id, count() as n) for points [group: (metric_id, n)]
