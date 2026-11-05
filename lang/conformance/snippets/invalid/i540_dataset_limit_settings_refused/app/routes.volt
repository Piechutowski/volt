// spec: §settings-whitelists, §dataset-settings, a dataset takes no default_limit: or max_limit:, as a route takes none
// want: setting "max_limit" is not valid on a dataset (§settings-whitelists); valid: strip, only, except
package app

import (
	db
)

scope /ms {
	dataset db.recent [strip: "ms_", max_limit: 20, default_limit: 5]
}
