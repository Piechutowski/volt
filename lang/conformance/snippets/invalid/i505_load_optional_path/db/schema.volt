package db

table posts {
  id           int       [pk]
  title        string
  published_at time.Time
}

table post_tags {
  post_id int    [pk]
  tag     string [pk]
}

ref: post_tags.post_id > posts.id [as: post, reverse: tags]

select feed for posts where exists tags (tag = :tag?) and (published_at < :at? or (published_at = :at? and id < :id?)) [order: (published_at desc, id desc), limit: :limit]
select picked for posts where id in :ids? [order: (id asc)]
