// spec: §predicates, an optional parameter does not stand under not, where its vanished term would turn false
// want: not stands over the optional parameter :min?
package db

table posts {
  id    int [pk]
  views int
}

select quiet for posts where not (views >= :min?)
