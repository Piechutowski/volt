// spec: §selects-over-groups — group: names projected items
// want: which the select does not project
package db

table points {
  id        int     [pk]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

select rows (metric_id, count() as n) for points [group: (ts)]
