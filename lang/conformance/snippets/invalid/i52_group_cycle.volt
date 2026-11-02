// spec: §groups — group references must be acyclic
package db

table ms_revenue {
  id int [pk]
}

group a = b + ms_revenue
group b = a
