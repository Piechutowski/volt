// spec: §selects-over-groups — an alias names no column of the target
// want: names a column of "points"
package db

Table points {
  id        int     [pk, increment]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

Select rows (count() as ts) for points
