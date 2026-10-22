// spec: §columns tag extension — tag keys are unique per column
Table users {
  id   int [pk]
  name string [tag: "json:\"a\"", tag: "json:\"b\""]
}
