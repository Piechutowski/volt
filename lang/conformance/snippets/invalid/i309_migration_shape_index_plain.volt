// spec: §migrations, §indexes, a shape body follows §indexes as a table does: a plain single-column index there is the column setting, and two of its indexes share no name
// want: is a plain single-column index: write the column setting index on "owner"
package db

table accounts {
  id    int    [pk]
  owner string [index]
  email string [unique]
}

migration 0001__create_accounts {
  create table accounts {
    id    int    [pk]
    owner string
    email string [unique]

    indexes {
      owner
      email [name: "idx_accounts_email", where: `email != ''`]
    }
  }
}
