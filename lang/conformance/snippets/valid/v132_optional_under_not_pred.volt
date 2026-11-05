// spec: §predicates, an optional parameter reached through a pred under not is taken out with the not
package db

table posts {
  id    int [pk]
  views int
}

pred popular { views >= :min? }

select quiet for posts where not popular
