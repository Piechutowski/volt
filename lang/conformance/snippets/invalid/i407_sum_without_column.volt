// spec: §selects-over-groups — only count counts the rows
// want: sum takes a column
package db

Table points {
  id        int     [pk, increment]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

Select rows (sum() as total) for points
