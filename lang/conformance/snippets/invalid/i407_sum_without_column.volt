// spec: §selects-over-groups — only count counts the rows
// want: sum takes a column
package db

table points {
  id        int     [pk]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

select rows (sum() as total) for points
