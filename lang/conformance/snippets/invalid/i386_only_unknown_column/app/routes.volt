// spec: §request-bodies — only: names columns the write casts
// want: which db.SettingUpdate does not cast
package app

import (
	db
)

Scope / {
	patch /settings/:id(int) Settings.Save [body: db.SettingUpdate(id), only: (colour)]
}
