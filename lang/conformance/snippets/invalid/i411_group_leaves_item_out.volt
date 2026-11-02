// spec: §selects-over-groups — group: lists every non-aggregate item
// want: leaves out label
package db

Table points {
  id        int     [pk, increment]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

Select rows (metric_id, label, count() as n) for points [group: (metric_id)]
