// closure table: every ancestor/descendant pair, including depth 0 self-pairs
Table categories {
  id int [pk]
  name string
}
Table category_paths [note: "Closure table over categories"] {
  ancestor_id int [ref: categories.id]
  descendant_id int [ref: categories.id]
  depth int [note: "0 for the self-pair"]

  Indexes {
    (ancestor_id, descendant_id) [pk]
    descendant_id
  }
}
