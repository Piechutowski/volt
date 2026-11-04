// spec: §dataset-settings, limit: and max_limit: on a dataset give every member route's limit from the query string a default and a most
package app

import (
	db
)

scope /ms {
	dataset db.recent [strip: "ms_", limit: 5, max_limit: 20]
}
