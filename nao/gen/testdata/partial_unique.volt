// a unique index with where: constrains only the rows its condition
// admits, so it yields no GetBy; the one without where: yields GetByHandle

table authors {
  id         int [pk]
  email      string
  handle     string [unique]
  deleted_at time.Time [null]

  indexes {
    email [unique, where: `deleted_at IS NULL`]
  }
}
