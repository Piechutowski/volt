// spec: §indexes, a [pk] index takes no name: or where:
// want: which it does not take
Table post_tags {
  post_id    int
  tag        string
  deleted_at time.Time [null]

  indexes {
    (post_id, tag) [pk, name: 'pk_post_tags', where: `deleted_at IS NULL`]
  }
}
