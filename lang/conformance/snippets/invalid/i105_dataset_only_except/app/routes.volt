// spec: §V13 — i105_dataset_only_except
// want: cannot both be set
package app

import (
	db
)

Scope /ms {
	dataset db.browse [only: (ms_revenue), except: (ms_usage)]
}
