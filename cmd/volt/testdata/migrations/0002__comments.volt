package blog

Migration 0002__comments {
  alter table posts {
    add views int [default: 0]
  }
  create index posts views
  create table comments {
    id      int    [pk, increment]
    post_id int    [ref: > posts.id]
    body    string
  }
}
