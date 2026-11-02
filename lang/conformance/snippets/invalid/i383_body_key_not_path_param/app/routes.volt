// spec: §request-bodies — the key of an update body is a path parameter of the route or a literal
// want: must be a path parameter of the route
package app

import (
	db
)

scope / {
	patch /settings Settings.Save [body: db.SettingUpdate(id)]
}
