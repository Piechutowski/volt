// spec: §selects-over-groups, a join of a relation of many rows repeats the target's row for each of them, so a limit's order names a key of the joined table too
// want: the join of "comments" repeats a row of posts for each of its rows, and order: names no key of comments
package db

table posts {
  id    int    [pk]
  title string
}

table comments {
  id      int    [pk]
  post_id int    [ref: posts.id]
  body    string
}

select replies (id, title, comments.body) for posts join comments [order: (id asc), limit: 5]
