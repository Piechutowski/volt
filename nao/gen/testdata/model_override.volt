// [model:] (extension) pins the Go model name where the inflector
// cannot know better: "menus" strips to "menu" only by English knowledge,
// and "order_status" is a singular the plural-table convention never fits.

table menus [model: "Menu"] {
  id int [pk]
  title string
  price float64 [default: 0]
}

// One row per status a customer order can be in
table order_status [model: "OrderStatus"] {
  code string [pk]
  label string [unique]
}
