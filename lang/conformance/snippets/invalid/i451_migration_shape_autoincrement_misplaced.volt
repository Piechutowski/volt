// spec: §migrations, §columns, a create table or rebuild table shape takes autoincrement on its database-assigned key alone: not on a text key, a column outside the key or a member of a composite key
// want: only the database-assigned key, the single-column primary key of the built-in int that is no foreign key, takes AUTOINCREMENT
package db

table tags {
  slug string [pk]
  hits int    [null]
}

table post_tags {
  post_id int    [pk]
  tag     string [pk]
}

migration 0001__init {
  create table tags {
    slug string [pk, autoincrement]
    hits int    [null]
  }
  create table post_tags {
    post_id int    [pk, autoincrement]
    tag     string [pk]
  }
}

migration 0002__hits [irreversible] {
  rebuild table tags {
    slug string [pk]
    hits int    [null, autoincrement]
  }
}
