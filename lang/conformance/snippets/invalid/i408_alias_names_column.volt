// spec: §selects-over-groups — an alias names no column of the target
// want: names a column of "points"
package db

table points {
  id        int     [pk, increment]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

select rows (count() as ts) for points
