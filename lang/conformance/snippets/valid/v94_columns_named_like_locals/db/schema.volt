// spec: §generated-query-surface, §predicates, key columns and select parameters named like the generated methods' locals and helpers once were, or like the predeclared names they use
package db

Table ledgers {
  err            int [pk]
  res            int [pk]
  n              int [pk]
  out            int [pk]
  rows           int [pk]
  q              int [pk]
  volt_err       int [pk]
  nil            int [pk]
  append         int [pk]
  ledger_scan    int [pk]
  ledger_get_sql int [pk]
  note           string
}

Table entries {
  row        int    [pk]
  err        string [unique]
  res        string [unique]
  n          int    [unique]
  out        string [unique]
  rows       string [unique]
  q          string [unique]
  v          string [unique]
  query      string [unique]
  args       string [unique]
  cols       string [unique]
  entry_scan string [unique]
  found      bool
}

Table volt_keys {
  volt_row int [pk]
  label    string
}

Table marks {
  nil   int [pk]
  label string
}

Table tallies {
  append int [pk]
  label  string
}

Select by_locals for entries where err = :err and res = :rows and n in :out and v = :v and query = :query and args = :args
Select by_helpers for entries where err = :entryScan and n in :entryByHelpersSQL
