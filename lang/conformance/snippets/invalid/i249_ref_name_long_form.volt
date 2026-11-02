// spec: §relationships-ref, a Ref carries no name and has no braced form
// want: expected ':' after Ref
table users { id int [pk] }
table posts { user_id int }
ref fk_posts_user: posts.user_id > users.id
ref {
  posts.user_id > users.id
}
