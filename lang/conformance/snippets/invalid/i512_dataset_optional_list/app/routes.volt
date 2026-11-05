// spec: §dataset-declaration, a dataset over a select with an optional list parameter is no route, as a query route to it is none
// want: select db.picked takes the optional list parameter "ids", which no dataset's route carries: a missing key and an empty list cannot be told apart in a query string, and an absent list matches every row where an empty one matches none; call the select from Go
package app

import (
	db
)

scope /ms {
	dataset db.picked [strip: "ms_"]
}
