// spec: §predicates, a parameter is optional at every use in a select, the preds it uses included, or at none
// want: a parameter is optional at every use in a select, the preds it uses included, or at none
package db

table posts {
  id int [pk]
  n  int
}

pred after { id > :id? }

select page for posts where after and n < :id
