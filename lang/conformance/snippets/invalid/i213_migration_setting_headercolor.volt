// spec: §migrations, headercolor: describes the diagram and is not a migration setting
// want: headercolor: describes the diagram
package db

Table users [headercolor: #3498db] {
  id int [pk]
}

Migration 0001__init {
  create table users {
    id int [pk, headercolor: #3498db]
  }
}
