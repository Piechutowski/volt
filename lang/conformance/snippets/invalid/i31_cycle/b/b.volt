package b

import (
	a
)

table users { id int [pk] }

scope /b { resources a.users }
