package app

import (
	migrations/db
)

scope /api {
	resources db.posts [default]
}
