// spec: §columns, a member of a composite primary key never carries null, as a single key column does not
// want: carries pk and null
table post_tags {
  post_id int    [pk]
  tag     string [pk, null]
}
