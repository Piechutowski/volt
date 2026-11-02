// spec: §predicates: a declared type stored as []byte cannot take a list parameter
// want: stored as []byte; it cannot take a list parameter
package db

type blob.Blob []byte from "example.com/app/blob"

table posts {
  id   int       [pk]
  icon blob.Blob
}

select byicon for posts where icon = :icon
select picked for posts where icon in :icons
