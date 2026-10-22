// spec: §migrations, §columns, a key column of a migration never carries null, a member of a composite key among them
// want: a key column is never NULL
package db

Table tags {
  id int [pk]
}

Table post_tags {
  post_id int    [pk]
  tag     string [pk]
}

Migration 0001__init {
  create table tags {
    id int [pk, null]
  }
  create table post_tags {
    post_id int    [pk]
    tag     string [pk, null]
  }
}
