// spec: §relationships-ref, a Ref carries no name and has no braced form
// want: expected ':' after Ref
Table users { id int [pk] }
Table posts { user_id int }
Ref fk_posts_user: posts.user_id > users.id
Ref {
  posts.user_id > users.id
}
