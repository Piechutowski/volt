// spec: §dataset-declaration — i106_dataset_unqualified: a bare dataset names a select of this package, which declares none; the imported package's must be qualified
// want: declares no select
package app

import (
	db
)

scope /ms {
	dataset browse
}
