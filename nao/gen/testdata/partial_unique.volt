// a unique index with where: constrains only the rows its condition
// admits, so it yields no GetBy; the one without where: yields GetByHandle
Table authors {
  id         int [pk, increment]
  email      string
  handle     string [unique]
  deleted_at time.Time [null]

  Indexes {
    email [unique, where: `deleted_at IS NULL`]
  }
}
