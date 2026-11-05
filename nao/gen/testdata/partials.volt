// partial-composed tables: injected columns and doc comments travel; the
// table-level override of deleted_at (with its own doc comment) wins (§tablepartial)

table_partial timestamps {
  // Set on insert
  created_at time.Time [default: `CURRENT_TIMESTAMP`]
  // NULL until first update
  updated_at time.Time [null]
}
table_partial soft_delete {
  // Soft-delete marker from the partial
  deleted_at time.Time [null]
}
table invoices {
  id int [pk]
  total float64
  ~timestamps
  ~soft_delete
  // Override: also records who deleted, see audit log
  deleted_at time.Time [null]
}
