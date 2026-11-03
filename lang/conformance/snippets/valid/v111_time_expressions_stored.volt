// spec: §default-values, §records-sample-data, expressions that write the stored form of a time.Time: CURRENT_TIMESTAMP, datetime() with modifiers that keep its form and zone, parenthesized or not
table posts {
  id   int       [pk]
  a    time.Time [default: `(CURRENT_TIMESTAMP)`]
  b    time.Time [default: `DateTime ('now', 'start of day', '+1 day')`]
  c    time.Time [default: `datetime(1700000000, 'unixepoch')`]
  d    time.Time [default: `(('2024-01-15 10:30:00'))`]
  e    time.Time [null]

  records (id, a, b, c, d, e) {
    1, `current_timestamp`, `datetime('2024-01-15T12:30:00+02:00')`, `datetime('now', '-' || 7 || ' days')`, "2024-01-15 10:30:00", `NULL`
  }
}
