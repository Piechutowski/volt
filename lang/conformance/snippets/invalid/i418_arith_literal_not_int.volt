// spec: §selects-over-groups — a literal is representable in the expression type
// want: does not fit int
package db

Table points {
  id        int     [pk, increment]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

Select rows (ts / 1.5 as half) for points
