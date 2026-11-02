// spec: §request-bodies — only: names each column once
// want: names "refresh" twice
package app

import (
	db
)

Scope / {
	patch /settings/:id(int) Settings.Save [body: db.SettingUpdate(id), only: (refresh, refresh)]
}
