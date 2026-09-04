// spec: §V13 — i104_dataset_strip
// want: is not a prefix of member table
package app

import (
	db
)

Scope /ms {
	dataset db.browse [strip: 'xx_']
}
