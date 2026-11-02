// spec: §request-bodies — an update body gives one argument per key column
// want: gives 0 argument(s)
package app

import (
	db
)

Scope / {
	patch /settings Settings.Save [body: db.SettingUpdate]
}
