// spec: §selects-over-groups — arithmetic takes numeric columns
// want: arithmetic takes numeric columns
package db

Table points {
  id        int     [pk, increment]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

Select rows (label + 1 as next) for points
