// spec: §datasets — i103_dataset_unknown
// want: no select
package app

import (
	db
)

scope /ms {
	dataset db.brows
}
