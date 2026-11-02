// spec: §request-bodies — a path parameter bound to a key spells its Go type
// want: is string but db.SettingUpdate takes int
package app

import (
	db
)

Scope / {
	patch /settings/:id Settings.Save [body: db.SettingUpdate(id)]
}
