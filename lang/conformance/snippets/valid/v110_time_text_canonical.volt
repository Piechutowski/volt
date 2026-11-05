// spec: §default-values, §records-sample-data, canonical date/time text in a record, in a SQL string default and in a SQL string record value, and expression defaults SQLite judges
table posts {
  id         int       [pk]
  posted_at  time.Time [default: `'2024-01-15 10:30:00'`]
  created_at time.Time [default: `CURRENT_TIMESTAMP`]
  due_at     time.Time [default: `datetime('now', '+1 day')`]

  records (id, posted_at, created_at, due_at) {
    1, "2024-01-15 10:30:00", "2024-01-15 10:30:00.5", `'2024-01-16 10:30:00'`
    2, "0001-01-01 00:00:00", `CURRENT_TIMESTAMP`, `datetime('now')`
  }
}
