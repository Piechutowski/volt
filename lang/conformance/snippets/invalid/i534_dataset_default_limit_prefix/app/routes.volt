// spec: §dataset-settings, default_limit: on a dataset whose limit a prefix parameter binds, always present, governs nothing
// want: default_limit: is the limit a request that leaves the key out gets, and the route binds no select's limit: parameter from the query string
package app

import (
	db
)

scope /ms/:limit(int) {
	dataset db.recent [strip: "ms_", default_limit: 5]
}
