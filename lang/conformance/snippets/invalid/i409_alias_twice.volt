// spec: §selects-over-groups — an alias appears once
// want: appears twice in the projection
package db

Table points {
  id        int     [pk, increment]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

Select rows (count() as n, sum(value) as n) for points
