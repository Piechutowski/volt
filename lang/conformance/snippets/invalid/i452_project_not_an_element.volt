// spec: §program-structure, §notes-become-doc-comments, project is not an element: a package documents itself in a hand-written Go file
// want: unknown element type "project"
project blog [note: "Blogging platform"] {
  owner: "platform team"
}

table posts {
  id int [pk]
}
