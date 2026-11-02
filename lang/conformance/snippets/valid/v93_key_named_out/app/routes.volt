package app

import (
	db
)

scope / {
	get /usage/:out(int) db.MsUsageGet
	get /usage           db.MsUsageByErr
}
