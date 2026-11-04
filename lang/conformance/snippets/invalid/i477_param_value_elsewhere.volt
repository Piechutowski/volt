// spec: §settings-whitelists, a parameter is a setting value only for a select's limit:
// want: timeout: takes a positive integer of seconds
package db

table page_views {
  id   int [pk]
  hits int
}

scope /api {
  get /views PageViewList [timeout: :seconds]
}
