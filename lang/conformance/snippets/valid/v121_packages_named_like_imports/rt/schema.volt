// spec: §imports, §generated-client, packages named like the client's own imports route their own tables, their clients import them as volt and the name, and another package imports one under an alias
package rt

table posts {
  id    int    [pk]
  title string
}

select feed for posts where title = :title? [order: (id asc)]

scope /rt [name: rt] {
  get  /feed  PostFeed
  post /posts PostCreate
  post /raw   Posts.Raw [name: raw, body: PostCreate]
}
