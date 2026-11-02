// spec: §datasets — i104_dataset_strip
// want: is not a prefix of member table
package app

import (
	db
)

scope /ms {
	dataset db.browse [strip: "xx_"]
}
