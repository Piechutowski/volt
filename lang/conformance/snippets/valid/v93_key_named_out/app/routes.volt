package app

import (
	db
)

Scope / {
	get /usage/:out(int) db.MsUsageGet
	get /usage           db.MsUsageByErr
}
