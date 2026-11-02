// spec: §selects-over-groups — group: lists the items that are not aggregates
// want: names the aggregate
package db

Table points {
  id        int     [pk, increment]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

Select rows (metric_id, count() as n) for points [group: (metric_id, n)]
