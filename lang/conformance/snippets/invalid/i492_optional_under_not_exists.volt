// spec: §predicates, an exists term that reads an optional parameter vanishes whole, so it does not stand under not
// want: not stands over the optional parameter :tag?
package db

table posts {
  id int [pk]
}

table post_tags {
  post_id int    [pk]
  tag     string [pk]
}

ref: post_tags.post_id > posts.id [as: post, reverse: tags]

select untagged for posts where not exists tags (tag = :tag?)
