// spec: §datasets — i107_dataset_not_member
// want: is not a member of select
package app

import (
	db
)

scope /ms {
	dataset db.browse [except: (users)]
}
