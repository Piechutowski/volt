// spec: §dataset-declaration, a scope prefix parameter, always present, cannot bind an optional parameter of the dataset's select
// want: parameter "year" of select db.browse is optional, and the scope prefix parameter "year" is a path segment, always present; declare the dataset in a scope without that prefix parameter, so the key binds from the query string, or name a second select whose parameter is required
package app

import (
	db
)

scope /years/:year(int) {
	dataset db.browse [strip: "ms_"]
}
