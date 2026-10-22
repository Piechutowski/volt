// spec: §migrations, §columns, a key column of a migration never carries null
// want: a key column is never NULL
package db

Table tags {
  id int [pk]
}

Table post_tags {
  post_id int
  tag     string
  Indexes {
    (post_id, tag) [pk]
  }
}

Migration 0001__init {
  create table tags {
    id int [pk, null]
  }
  create table post_tags {
    post_id int
    tag     string [null]
    Indexes {
      (post_id, tag) [pk]
    }
  }
}
