// spec: §request-bodies — an update body gives one argument per key column
// want: gives 0 argument(s)
package app

import (
	db
)

scope / {
	patch /settings Settings.Save [body: db.SettingUpdate]
}
