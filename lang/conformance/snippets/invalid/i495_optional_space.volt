// spec: §predicates, the ? of an optional parameter stands directly after its name
// want: '?' makes a parameter optional only directly after its name, :id?
package db

table posts {
  id int [pk]
}

select page for posts where id < :id ?
