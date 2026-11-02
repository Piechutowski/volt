// spec: §namespaces-and-uniqueness, declared Type names are unique within the package
// want: duplicate type name
type Slug string
type Slug string
table t {
  s Slug
}
