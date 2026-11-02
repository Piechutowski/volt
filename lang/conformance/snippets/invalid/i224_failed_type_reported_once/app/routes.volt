package app

import (
  db
)

scope /api {
  get /top db.PostTop [name: Top]
  dataset db.top
}
