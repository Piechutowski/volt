// spec: §selects-over-groups — an expression names a column, which types it
// want: an expression names a column
package db

Table points {
  id        int     [pk, increment]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

Select rows (1 + :x as y) for points
