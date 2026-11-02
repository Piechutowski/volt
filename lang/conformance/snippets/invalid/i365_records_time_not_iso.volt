// spec: §records-sample-data, a time.Time record value is ISO 8601 text
// want: must be ISO 8601 text
table posts {
  id        int       [pk]
  posted_at time.Time
}

records posts(id, posted_at) {
  1, "2024-01-15 10:30:00"
  2, "not a date at all"
  3, "2024-01-15T10:30:00Z"
}
