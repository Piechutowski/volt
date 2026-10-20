// partial-composed tables: injected columns and notes travel; the
// table-level override of deleted_at (with its own note) wins (§tablepartial)
TablePartial timestamps {
  created_at time.Time [not null, default: `now()`, note: 'Set on insert']
  updated_at time.Time [note: 'NULL until first update']
}
TablePartial soft_delete {
  deleted_at time.Time [note: 'Soft-delete marker from the partial']
}
Table invoices {
  id int [pk]
  total float64 [not null]
  ~timestamps
  ~soft_delete
  deleted_at time.Time [note: 'Override: also records who deleted, see audit log']
}
