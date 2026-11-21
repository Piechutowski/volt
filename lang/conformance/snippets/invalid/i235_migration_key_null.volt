// spec/migrations, spec/columns, a key column of a migration never carries null, a member of a composite key among them
// want: a key column is never NULL
package db

table tags {
  id int [pk]
}

table post_tags {
  post_id int    [pk]
  tag     string [pk]
}

migration 0001__init {
  table_create tags {
    id int [pk, null]
  }
  table_create post_tags {
    post_id int    [pk]
    tag     string [pk, null]
  }
}
