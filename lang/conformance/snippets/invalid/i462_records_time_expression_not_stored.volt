// spec: §records-sample-data, a time.Time record value written as an expression is one SQLite evaluates to the stored form
// want: must be an expression SQLite evaluates to the stored form of a time.Time
table posts {
  id        int       [pk]
  posted_at time.Time
}

records posts(id, posted_at) {
  1, `datetime('now', 'subsec')`
  2, `strftime('%Y-%m-%d %H:%M:%f', 'now')`
  3, `(('2024-01-15T10:30:00Z'))`
  4, `date('now')`
}
