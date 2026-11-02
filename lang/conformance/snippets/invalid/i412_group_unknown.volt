// spec: §selects-over-groups — group: names projected items
// want: which the select does not project
package db

Table points {
  id        int     [pk, increment]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

Select rows (metric_id, count() as n) for points [group: (ts)]
