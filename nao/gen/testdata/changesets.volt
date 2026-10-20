// Changesets (spec §changesets): required over every built-in storage
// kind, enum membership on a plain and a nullable enum column, a
// composite key with no Update, and a table whose only column is the
// auto-increment key, so Cast and From have nothing to set.
package metrics

Enum visibility {
  public
  unlisted
}

Table posts {
  id       int [pk, increment]
  slug     string [not null, required, unique]
  words    int [not null, required]
  score    float64 [not null, required]
  cover    []byte [not null, required]
  state    visibility [not null, default: visibility.public]
  reviewed visibility
}

Table post_tags {
  post_id int [not null, ref: > posts.id]
  tag     string [not null]

  indexes {
    (post_id, tag) [pk]
  }
}

Table pings {
  id int [pk, increment]
}
