// spec: §dataset-settings, max_limit: on a dataset whose select takes no limit: parameter governs nothing
// want: max_limit: is the most a request may ask, and the route binds no select's limit: parameter
package app

import (
	db
)

scope /ms {
	dataset db.browse [strip: "ms_", max_limit: 50]
}
