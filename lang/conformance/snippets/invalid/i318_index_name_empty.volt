// spec: §indexes, a written name is not empty: an empty name: neither names the index nor falls back to the derived name, and would let an expression key escape its name: rule
// want: has an empty name:
Table posts {
  id    int    [pk, increment]
  a     int
  b     int
  title string

  Indexes {
    (a, b) [name: ""]
    `lower(title)` [name: ""]
  }
}
