// spec: §types — the storage type must be a built-in
// want: is not a built-in
type Slug text
table t {
  s Slug
}
