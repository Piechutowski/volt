// spec: §records-sample-data — in-table records with implicit column list
Table comments {
  id int [pk]
  user_id int
  title string

  Records {
    1, 2, "First comment"
  }
}
