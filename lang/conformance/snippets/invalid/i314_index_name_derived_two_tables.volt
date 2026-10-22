// spec: §indexes, derived names are unique in the schema too: idx_<table>_<col> joins names with underscores, so two tables can derive one name with no name: written at all
// want: tables "a_b" and "a" each have an index named "idx_a_b_c"
Table a_b {
  id int    [pk]
  c  string [index]
}

Table a [model: "Alpha"] {
  id  int    [pk]
  b_c string [index]
}

Table post {
  id       int    [pk, increment]
  tag_slug string [unique]
}

Table post_tag [model: "Label"] {
  id   int    [pk, increment]
  slug string [index]
}
