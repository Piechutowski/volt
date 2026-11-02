// spec: §columns, a partial's autoincrement column refused where it joins a composite key is reported once per table, naming the table and its key, and stays valid in a table where it is the whole key
// want: autoincrement on "id" in table "a", whose primary key is (id, site)
table_partial base {
  id int [pk, autoincrement]
}

table a {
  ~base
  site string [pk]
}

table b {
  ~base
  site string [pk]
}

table c {
  ~base
  title string
}
