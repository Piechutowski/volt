// spec: §keywords, §program-structure, a word that opens an element has one capitalized spelling: TABLE and table are errors naming Table
// want: write Table, not TABLE
TABLE users {
  id int [pk]
}

table posts {
  id int [pk]
}

type Status string
