// spec: §indexes, derived names are unique in the schema too: idx_<table>_<col> joins names with underscores, so two tables can derive one name with no name: written at all
// want: tables "a_b" and "a" each have an index named "idx_a_b_c"
table a_b {
  id int    [pk]
  c  string [index]
}

table a [model: "Alpha"] {
  id  int    [pk]
  b_c string [index]
}

table post {
  id       int    [pk]
  tag_slug string [unique]
}

table post_tag [model: "Label"] {
  id   int    [pk]
  slug string [index]
}
