// spec: §selects-over-groups — the reverse relation of a self-reference takes the table's own name
// want: takes the table's own name
package db

table metrics {
  id        int [pk, increment]
  name      string
  parent_id int [null, ref: metrics.id]
}

select rows (id, metrics.name) for metrics join metrics
