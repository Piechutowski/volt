package blog

Migration 0001__create_posts {
  create table posts {
    id    int    [pk, increment]
    title string [not null]
    body  string
  }
}
