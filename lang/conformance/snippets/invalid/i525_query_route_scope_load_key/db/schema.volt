package db

table users {
  id    int    [pk]
  email string [unique]
}

table posts {
  id        int    [pk]
  author_id int    [ref: users.id]
  title     string
}

select by for posts where author_id = :user [order: (id asc)]
