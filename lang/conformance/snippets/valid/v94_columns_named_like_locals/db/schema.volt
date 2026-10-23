// spec: §generated-query-surface, §predicates, key columns and select parameters named like the generated methods' locals once were
package db

Table ledgers {
  err      int [pk]
  res      int [pk]
  n        int [pk]
  out      int [pk]
  rows     int [pk]
  q        int [pk]
  volt_err int [pk]
  note     string
}

Table entries {
  row   int    [pk]
  err   string [unique]
  res   string [unique]
  n     int    [unique]
  out   string [unique]
  rows  string [unique]
  q     string [unique]
  v     string [unique]
  query string [unique]
  args  string [unique]
  cols  string [unique]
  found bool
}

Table volt_keys {
  volt_row int [pk]
  label    string
}

Select by_locals for entries where err = :err and res = :rows and n in :out and v = :v and query = :query and args = :args
