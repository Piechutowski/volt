// spec: §selects-over-groups, a table without a primary key whose unique column is never NULL has that column as its key, and the tie names it
// want: order: names no key of tags, so two rows can tie and the ones kept at the cut would be arbitrary; end it in a key of tags, (..., slug asc)
package db

table tags {
  slug string [unique]
  hits int
}

select top for tags [order: (hits desc), limit: 5]
