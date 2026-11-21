// spec/columns, spec/validators, spec/query-routes (an unknown type reported once, at the column)
// want: unknown type "int64"
package app

import (
  db
)

scope /api {
  get /authors/:id(int) db.AuthorGet
  resources db.authors [default]
  get /ranked db.AuthorRanked
}
