// spec: §selects-over-groups, under group: a key of the target in group: does not make the order total once a group: item reaches a relation of many rows
// want: does not name the group: item "commenter"
package db

table posts {
  id    int    [pk]
  title string
}

table comments {
  id        int [pk]
  post_id   int [ref: posts.id]
  member_id int
}

select per_commenter (id, comments.member_id as commenter, count() as n) for posts join comments [group: (id, commenter), order: (n desc, id asc), limit: 5]
