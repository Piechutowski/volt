// [model:] (extension) pins the Go model name where the inflector
// cannot know better: "menus" strips to "menu" only by English knowledge,
// and "order_status" is a singular the plural-table convention never fits.
Table menus [model: "Menu"] {
  id int [pk, increment]
  title string
  price float64 [default: 0]
}

Table order_status [model: "OrderStatus", note: "One row per status a customer order can be in"] {
  code string [pk]
  label string [unique]
}
