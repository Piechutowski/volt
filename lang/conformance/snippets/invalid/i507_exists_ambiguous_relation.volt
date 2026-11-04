// spec: §predicates, §relationships-ref, exists names a relation as join does, so a reverse name two relationships share is ambiguous until reverse: names one
// want: relation "posts" of table "members" is ambiguous
package db

table members {
  id   int    [pk]
  name string
}

table posts {
  id        int [pk]
  author_id int [ref: members.id]
  editor_id int [null, ref: members.id]
}

select active for members where exists posts (id > 0)
