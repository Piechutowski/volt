// spec: §predicates, an optional parameter reached through a pred under not is refused too
// want: not stands over the optional parameter :min?
package db

table posts {
  id    int [pk]
  views int
}

pred popular { views >= :min? }

select quiet for posts where not popular
