package blog

migration 0001__create_posts {
  create table posts {
    id    int    [pk]
    title string
    body  string [null]
  }
}
