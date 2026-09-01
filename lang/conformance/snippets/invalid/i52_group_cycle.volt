// spec: §V9.4 — group references must be acyclic
package db

Table ms_revenue {
  id integer [pk]
}

Group a = b + ms_revenue
Group b = a
