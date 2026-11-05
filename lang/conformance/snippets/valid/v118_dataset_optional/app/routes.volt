// spec: §dataset-declaration, an optional parameter of a dataset's select binds from the query string alone, absent when the key is missing, beside a prefix parameter
package app

import (
	db
)

scope /orgs/:org {
	dataset db.browse [strip: "ms_"]
}
