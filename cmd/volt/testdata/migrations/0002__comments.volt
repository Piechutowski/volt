package blog

migration 0002__comments {
  table_alter posts {
    add views int [default: 0]
  }
  index_create posts views
  table_create comments {
    id      int    [pk]
    post_id int    [ref: posts.id]
    body    string
  }
}
