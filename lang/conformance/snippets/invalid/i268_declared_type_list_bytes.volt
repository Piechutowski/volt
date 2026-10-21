// spec: §predicates: a declared type stored as []byte cannot take a list parameter
// want: stored as []byte; it cannot take a list parameter
package db

Type blob.Blob []byte from 'example.com/app/blob'

Table posts {
  id   int       [pk, increment]
  icon blob.Blob
}

Select byicon for posts where icon = :icon
Select picked for posts where icon in :icons
