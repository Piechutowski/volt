// spec: §V9.3 — removing a table that is not a member, even inside a set, is an error
// want: is not a member
package db

Table ms_revenue {
  id integer [pk]
}

Table ms_usage {
  id integer [pk]
}

Table ms_dict {
  code text [pk]
}

Group series = (ms_revenue, ms_usage) \ (ms_usage, ms_dict)
