// spec: §imports
package a

import (
	b
)

table users { id int [pk] }

scope / { resources b.users }
