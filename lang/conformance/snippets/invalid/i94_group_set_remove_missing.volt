// spec: §groups — removing a table that is not a member, even inside a set, is an error
// want: is not a member
package db

Table ms_revenue {
  id int [pk]
}

Table ms_usage {
  id int [pk]
}

Table ms_dict {
  code string [pk]
}

Group series = (ms_revenue, ms_usage) \ (ms_usage, ms_dict)
