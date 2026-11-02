// spec: §datasets — i105_dataset_only_except
// want: cannot both be set
package app

import (
	db
)

scope /ms {
	dataset db.browse [only: (ms_revenue), except: (ms_usage)]
}
