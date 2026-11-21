package app

import (
	data
)

pipeline api {
	use volt.RequestID
}
pipeline dead {
	use volt.Logger
}

scope /api [pipe: api] {
	resources data.posts [default]
}
