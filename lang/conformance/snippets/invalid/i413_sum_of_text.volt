// spec: §selects-over-groups — sum takes a numeric column
// want: sum takes a numeric column
package db

table points {
  id        int     [pk, increment]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

select rows (sum(label) as total) for points
