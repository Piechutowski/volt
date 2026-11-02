// spec: §columns tag extension, §struct-tags — verbatim struct-tag passthrough;
// a json key replaces the generated default, others append in order
table users {
  id        int [pk]
  user_name string [tag: "json:\"userName\"", tag: "xml:\"user-name,attr\""]
  bio       string [tag: "msgpack:\"bio,omitempty\""]
}
