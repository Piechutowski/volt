// spec: §selects-over-groups — an ungrouped aggregate yields one row, which order: cannot sort
// want: which order: cannot sort
package db

Table points {
  id        int     [pk, increment]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

Select rows (count() as n) for points [order: (n desc)]
