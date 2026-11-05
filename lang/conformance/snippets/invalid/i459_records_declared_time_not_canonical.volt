// spec: §records-sample-data, a declared type stored as time.Time takes the canonical stored form too
// want: must be SQLite's canonical UTC date/time text, "2024-01-15 08:30:00"
type stamp.Stamp time.Time from "example.com/app/stamp"

table events {
  id int [pk]
  at stamp.Stamp

  records (id, at) {
    1, "2024-01-15 10:30:00+02:00"
  }
}
