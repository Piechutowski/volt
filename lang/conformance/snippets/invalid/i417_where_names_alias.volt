// spec: §selects-over-groups — where sees columns, never aliases
// want: is an alias, which where does not see
package db

Table points {
  id        int     [pk, increment]
  metric_id int
  ts        int
  value     float64
  label     string
  note      string  [null]
}

Select rows (metric_id, count() as n) for points where n > 1 [group: (metric_id)]
