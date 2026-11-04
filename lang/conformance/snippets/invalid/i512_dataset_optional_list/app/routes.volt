// spec: §dataset-declaration, a dataset over a select with an optional list parameter is no route, as a query route to it is none
// want: takes the optional list parameter "ids", which a route cannot carry
package app

import (
	db
)

scope /ms {
	dataset db.picked [strip: "ms_"]
}
