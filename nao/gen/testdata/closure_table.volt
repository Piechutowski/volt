// closure table: every ancestor/descendant pair, including depth 0 self-pairs

table categories {
  id int [pk]
  name string
}
// Closure table over categories
table category_paths {
  ancestor_id int [pk, ref: categories.id]
  descendant_id int [pk, index, ref: categories.id]
  // 0 for the self-pair
  depth int
}
