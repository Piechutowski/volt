// spec: §columns, §validators, §query-routes (an unknown type is reported once, at the column)
// want: unknown type "int64"
package app

import (
  db
)

Scope /api {
  get /authors/:id(int) db.AuthorGet
  resources db.authors [default]
  get /ranked db.AuthorRanked
}
