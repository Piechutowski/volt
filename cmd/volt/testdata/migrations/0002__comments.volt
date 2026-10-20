package blog

Migration 0002__comments {
  alter table posts {
    add views int [not null, default: 0]
  }
  create index posts views
  create table comments {
    id      int    [pk, increment]
    post_id int    [not null, ref: > posts.id]
    body    string [not null]
  }
}
