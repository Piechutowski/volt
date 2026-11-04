// spec: §route-loads, §dataset-settings, default_limit: and max_limit: take a positive integer that fits a 32-bit int, the limit they govern being an int on every target
// want: max_limit: takes a positive integer of rows that fits a 32-bit int, at most 2147483647
package app

import (
	db
)

scope /api {
	get /top db.PostTop [max_limit: 2147483648]
	dataset db.recent [strip: "ms_", default_limit: 2147483648]
}
