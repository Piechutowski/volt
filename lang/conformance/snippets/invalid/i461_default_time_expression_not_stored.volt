// spec: §default-values, an expression default of a time.Time column is one SQLite evaluates to the stored form: no 'subsec', no strftime, julianday or unixepoch, no 'localtime', and parentheses hide no SQL string
// want: must be an expression SQLite evaluates to the stored form of a time.Time
table posts {
  id   int       [pk]
  a    time.Time [default: `datetime('now', 'subsec')`]
  b    time.Time [default: `strftime('%Y-%m-%d %H:%M:%f', 'now')`]
  c    time.Time [default: `julianday('now')`]
  d    time.Time [default: `unixepoch()`]
  e    time.Time [default: `datetime('now', 'localtime')`]
  f    time.Time [default: `datetime('now') || ''`]
  g    time.Time [default: `('2024-01-15T10:30:00Z')`]
}
