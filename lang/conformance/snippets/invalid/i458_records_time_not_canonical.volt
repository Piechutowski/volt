// spec: §records-sample-data, a time.Time record value is the canonical stored form: no T, no Z, no offset, seconds always, no trailing zero in the fraction
// want: must be SQLite's canonical UTC date/time text, "2024-01-15 10:30:00"
table posts {
  id        int       [pk]
  posted_at time.Time
}

records posts(id, posted_at) {
  1, "2024-01-15T10:30:00Z"
  2, "2024-01-15 12:30:00+02:00"
  3, "2024-01-15 10:30"
  4, "2024-01-15"
  5, "2024-01-15 10:30:00.500"
  6, `'2024-01-15T10:30:00'`
}
