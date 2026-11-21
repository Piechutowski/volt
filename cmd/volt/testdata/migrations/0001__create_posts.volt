package blog

migration 0001__create_posts {
  table_create posts {
    id    int    [pk]
    title string
    body  string [null]
  }
}
