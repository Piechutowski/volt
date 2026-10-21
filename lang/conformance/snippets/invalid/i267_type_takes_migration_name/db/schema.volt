// spec: §types, §migration-tooling, §projects-and-packages, a bare declared type of a package that declares migrations is named apart from the generated Migrations and Migrate
// want: takes the Go name of the generated migration list
package db

Type Migrations string
Type Migrate    string

Table posts {
  id int        [pk, increment]
  a  Migrations
  b  Migrate
}

Migration 0001__posts {
  create table posts {
    id int    [pk, increment]
    a  string
    b  string
  }
}
