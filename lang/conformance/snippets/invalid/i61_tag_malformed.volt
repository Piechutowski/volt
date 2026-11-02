// spec: §columns tag extension — the value must be one key:"value" pair
table users {
  id   int [pk]
  name string [tag: "json=name"]
}
