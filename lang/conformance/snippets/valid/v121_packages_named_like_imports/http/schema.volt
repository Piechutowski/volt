package http

table pages {
  id   int    [pk]
  path string
}

scope /http [name: http] {
  get /pages     PageList
  get /pages/raw Pages.Raw [name: raw]
}
