package app

import (
	validation/db
)

scope /api {
	resources db.members [default]
}
