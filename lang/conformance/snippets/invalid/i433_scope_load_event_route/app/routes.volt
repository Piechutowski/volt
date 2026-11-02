// spec: §scope-loads — an event route runs no load
// want: an event route runs no load
package app

import (
	db
)

scope / {
	load user CurrentUser
	get /events volt.Events
}
