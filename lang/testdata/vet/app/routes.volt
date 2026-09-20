package app

import (
	data
)

Pipeline api {
	use volt.RequestID
}
Pipeline dead {
	use volt.Logger
}

Scope /api [pipe: api] {
	resources data.posts [default]
}
