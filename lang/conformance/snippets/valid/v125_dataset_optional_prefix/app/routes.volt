// spec: §dataset-declaration, a scope prefix parameter binds an optional parameter of the dataset's select as present
package app

import (
	db
)

scope /years/:year(int) {
	dataset db.browse [strip: "ms_"]
}
