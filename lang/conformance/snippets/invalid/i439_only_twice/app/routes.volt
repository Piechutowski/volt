// spec: §request-bodies — only: names each column once
// want: names "refresh" twice
package app

import (
	db
)

scope / {
	patch /settings/:id(int) Settings.Save [body: db.SettingUpdate(id), only: (refresh, refresh)]
}
