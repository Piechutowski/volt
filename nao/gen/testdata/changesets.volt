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
  slug     string [required, unique]
  words    int [required]
  score    float64 [required]
  cover    []byte [required]
  state    visibility [default: visibility.public]
  reviewed visibility [null, default: visibility.unlisted]
  rank     int [default: -1]
  draft    bool [default: true]
  caption  string [null, default: 'none']
  ratio    float64 [default: 0.5]
  weight   float64 [null, default: 1]
  scale    float64 [null, default: 2.0]
  kilo     float64 [null, default: 1e3]
  shift    float64 [null, default: -3]
  hits     int [null, default: 0, tag: 'json:"hits,omitempty"']
  views    int [default: 0]
  label    string [null, default: '']
  flag     bool [null, default: false]
  secret   string [null, tag: 'json:"-"']
}

Table post_tags {
  post_id int [ref: posts.id]
  tag     string

  indexes {
    (post_id, tag) [pk]
  }
}

Table pings {
  id int [pk, increment]
}
