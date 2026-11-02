// spec: §selects-over-groups — a derived item takes an alias
// want: a derived item takes an alias
package db

Table points {
  id        int     [pk, increment]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

Select rows (id, count()) for points
