// spec: §columns, §query-routes, §route-conflicts, §route-names-and-reverse-urls, §settings-whitelists, §dataset-settings (an unknown type silences only what depends on it)
// want: takes a request body
package app

import (
  db
)

Scope /api {
  get /authors/:id(int) db.AuthorUpdate
  delete /authors/:id(int)/:extra(int) db.AuthorDelete
  get /authors/:id... db.AuthorGet
  put /authors db.AuthorUpdate
  get /ranked db.AuthorRanked
  get /ranked Home.Index
  get /tops Home.AuthorRanked
  get /board db.AuthorRanked [name: Board, colour: red]
  get /ranked/:min db.AuthorRanked [name: RankedFrom]
  get /shown/:e/:zzz db.AuthorShown
  dataset db.ranked [only: (posts), colour: red]
  get /missing db.AuthorMissing
  dataset db.missing
  get /floor/:nope db.AuthorRanked [name: Floor]
  get /authors/:id(int) db.AuthorGet [colour: blue]
  get /authors/:id(int) Home.Show
}

Scope /v2 {
  resources db.authors [default, colour: red, only: (index, show, bogus)]
  get /authors/:key Home.Edit
  get /writers/:id(int) Home.Show [name: Author]
}
