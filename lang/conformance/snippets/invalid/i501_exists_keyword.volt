// spec: §keywords, §predicates, exists is a keyword inside a predicate and names no column there
// want: "exists" is a keyword inside a predicate and names no column there
package db

table flags {
  id     int  [pk]
  exists bool
}

select set for flags where exists = true
