// spec: §selects-over-groups — an expression names a column, which types it
// want: an expression names a column
package db

table points {
  id        int     [pk]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

select rows (1 + :x as y) for points
