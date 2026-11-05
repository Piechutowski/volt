// spec: §indexes, a written name is not empty: an empty name: neither names the index nor falls back to the derived name, and would let an expression key escape its name: rule
// want: has an empty name:
table posts {
  id    int    [pk]
  a     int
  b     int
  title string

  indexes {
    (a, b) [name: ""]
    `lower(title)` [name: ""]
  }
}
