// spec: §groups — removing a table that is not a member, even inside a set, is an error
// want: is not a member
package db

table ms_revenue {
  id int [pk]
}

table ms_usage {
  id int [pk]
}

table ms_dict {
  code string [pk]
}

group series = (ms_revenue, ms_usage) \ (ms_usage, ms_dict)
