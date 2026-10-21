// spec: §migrations, §relationships-ref, a migration column's set null needs null, set default needs null or a default:
// want: set null on foreign-key column
package db

Table users {
  id int [pk]
}

Table posts {
  id          int [pk]
  author_id   int [null]
  reviewer_id int [null]
}

Ref: posts.author_id > users.id [delete: set null]
Ref: posts.reviewer_id > users.id [update: set default]

Migration 0001__init {
  create table users {
    id int [pk]
  }
  create table posts {
    id          int [pk]
    author_id   int [ref: users.id, delete: set null]
    reviewer_id int [ref: users.id, update: set default]
  }
}
