// spec: §notes, §types, a declared type carries no note: its Go declaration documents it
type Slug string [note: "a URL segment"]
table posts {
  id int [pk]
}
