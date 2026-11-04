// spec: §dataset-declaration, a scope prefix parameter, always present, cannot bind an optional parameter of the dataset's select
// want: is optional, and a path segment is always present
package app

import (
	db
)

scope /years/:year(int) {
	dataset db.browse [strip: "ms_"]
}
