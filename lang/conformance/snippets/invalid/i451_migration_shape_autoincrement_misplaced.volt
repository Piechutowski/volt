// spec: §migrations, §columns, a create table or rebuild table shape takes autoincrement on its rowid alias alone: not on a text key, a column outside the key or a member of a composite key
// want: only the rowid alias, the single-column int primary key, takes AUTOINCREMENT
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
