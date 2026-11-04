// spec: §dataset-declaration, a dataset over a select with an optional list parameter routes it, as a query route to it does
package app

import (
	db
)

scope /ms {
	dataset db.picked [strip: "ms_"]
}
