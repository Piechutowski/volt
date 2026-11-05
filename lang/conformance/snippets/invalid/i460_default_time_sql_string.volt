// spec: §default-values, an expression default of a time.Time column that is one SQL string is the canonical stored form
// want: must be a SQL string of SQLite's canonical UTC date/time text, "2024-01-15 10:30:00"
table posts {
  id        int       [pk]
  posted_at time.Time [default: `'2024-01-15T10:30:00Z'`]
}
