// spec: §records-sample-data, a time.Time record value is date/time text
// want: must be date/time text in SQLite's canonical UTC form
table posts {
  id        int       [pk]
  posted_at time.Time
}

records posts(id, posted_at) {
  1, "2024-01-15 10:30:00"
  2, "not a date at all"
}
