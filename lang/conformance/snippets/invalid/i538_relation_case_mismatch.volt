// spec: §relationships-ref, §selects-over-groups, a relation is named exactly; one name that matches a single relation only without regard to case names no relation, and is no ambiguity
// want: table "posts" has no relation "Author"; did you mean "author"? names are case-sensitive
package db

table users {
  id   int    [pk]
  name string
}

table posts {
  id        int [pk]
  author_id int
}

ref: posts.author_id > users.id [as: author, reverse: written]

select by_name (id, Author.name) for posts join Author
select tagged for users where exists Written (id > 0)
