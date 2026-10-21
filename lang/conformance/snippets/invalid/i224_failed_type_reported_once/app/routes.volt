package app

import (
  db
)

Scope /api {
  get /top db.PostTop [name: Top]
  dataset db.top
}
