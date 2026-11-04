// spec: §route-loads, one query key binds several arguments, optional and required, of one load or of several, and each binds by its own parameter's type and rule
package app

import (
	db
)

scope / {
	get /mixed Pages.Mixed [name: mixed] {
		x db.PostTwo(q, q)
	}
	get /pair Pages.Pair [name: pair] {
		y db.PostPair(q, q)
		z db.PostTwo(q, q)
	}
	get /under Pages.Under [name: under] {
		x_y db.PostByZ(z)
		x   db.PostByYz(y_z)
	}
}
