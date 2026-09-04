// spec: §V11.6 — the data package whose default CRUD the resources bind
package db

Table users {
  id    integer [pk, increment]
  email text    [not null, unique]
}

Table tags {
  id   integer [pk, increment]
  name text    [not null]
}
