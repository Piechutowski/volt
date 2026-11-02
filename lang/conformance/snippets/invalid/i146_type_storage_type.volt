// spec: §types, the storage type is a built-in, never another declared type
// want: is not a built-in
type Slug string
type Handle Slug
table t {
  h Handle
}
