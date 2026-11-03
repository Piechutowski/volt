package app

import (
	tables/db
)

scope /api {
	resources db.notes [default]
}
