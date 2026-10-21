// spec: §columns — legacy bare pk / unique between type and settings
Table t {
  id int pk
  email string unique
  code int pk unique [not null]
}
