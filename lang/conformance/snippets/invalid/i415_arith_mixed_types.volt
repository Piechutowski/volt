// spec: §selects-over-groups — an expression has one type
// want: an expression has one type
package db

table points {
  id        int     [pk, increment]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

select rows (ts + value as mixed) for points
