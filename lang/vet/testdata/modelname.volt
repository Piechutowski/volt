// analyzers: modelname
table users {
  id int [pk]
}
table order_items {
  id int [pk]
}
table people {
  id int [pk]
}
table categories {
  id int [pk]
}
table addresses {
  id int [pk]
}
table person {
  id int [pk]
}
table menus [model: "Menu"] {
  id int [pk]
}
table account_statuses {
  id int [pk]
}
table order_status { //WANT modelname
  code string [pk]
}
table bonus { //WANT modelname
  id int [pk]
}
table analysis_results {
  id int [pk]
}
table axis { //WANT modelname
  id int [pk]
}
