// spec: §types, §migration-tooling, §projects-and-packages, a bare declared type of a package that declares migrations is named apart from the generated Migrations and Migrate
// want: takes the Go name of the generated migration list
package db

type Migrations string
type Migrate    string

table posts {
  id int        [pk]
  a  Migrations
  b  Migrate
}

migration 0001__posts {
  create table posts {
    id int    [pk]
    a  string
    b  string
  }
}
