// spec/selects-over-groups, spec/predicates, the limit: parameter is never optional
// want: a limit is never optional
package db

table posts {
  id int [pk]
}

select first for posts [order: (id asc), limit: :limit?]
