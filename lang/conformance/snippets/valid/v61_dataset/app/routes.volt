// spec: §V13 — a dataset expands a group select into one query route per member, stripping the table prefix
package app

import (
	db
)

Scope /ms {
	dataset db.browse [strip: 'ms_', except: (ms_notes)]
	get /notes Notes.Browse
}
