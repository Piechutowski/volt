// partial-composed tables: injected columns and notes travel; the
// table-level override of deleted_at (with its own note) wins (§tablepartial)
TablePartial timestamps {
  created_at time.Time [default: `now()`, note: 'Set on insert']
  updated_at time.Time [null, note: 'NULL until first update']
}
TablePartial soft_delete {
  deleted_at time.Time [null, note: 'Soft-delete marker from the partial']
}
Table invoices {
  id int [pk]
  total float64
  ~timestamps
  ~soft_delete
  deleted_at time.Time [null, note: 'Override: also records who deleted, see audit log']
}
