// spec: §indexes — pk and unique may be combined on one index (redundant but legal)
Table t {
  id int
  Indexes {
    id [pk, unique]
  }
}
