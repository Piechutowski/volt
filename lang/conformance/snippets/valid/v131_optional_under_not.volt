// spec: §predicates, a not over a term taken out is taken out with it, and a null test, which reads no parameter, is never taken out
package db

table posts {
  id    int [pk]
  views int
  seen  time.Time [null]
}

select quiet for posts where not (views >= :min?)
select unread for posts where views < :min? or seen is null
