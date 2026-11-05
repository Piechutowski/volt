// spec: §projects-and-packages, §generated-client, a package named http routes its own posts through query routes alone, and its client imports it as voltHttp
package http

table posts {
  id    int    [pk]
  title string [required]
}

scope /api [name: api] {
  get /posts         PostList
  get /posts/:id(int) PostGet
}
