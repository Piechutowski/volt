// spec/migrations, the words of a migration body are identifiers outside a Migration and names inside its operations
package db

table t {
  up     int [pk]
  to     string
  table  string
  create string
  drop   string
  index  string
  add    string
  exec   string
}

migration 0001__create_t {
  table_create t {
    up     int [pk]
    to     string
    table  string
    create string
    drop   string
    index  string [null]
    add    string
    exec   string
  }
  table_alter t {
    rename to to down
    drop index string [null]
    add rename string [null]
  }
}
