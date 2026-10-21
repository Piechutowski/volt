// spec: §records-sample-data — null and an empty field are allowed only on a nullable column
// want: is null, but the column is not nullable
Table t {
  id int [pk]
  n int [not null]
  records {
    1, 
  }
}
