package app

scope / [error_handler: Errors] {
	get /hello Pages.Hello [name: hello]
	get /broken Pages.Broken [name: broken]

	scope /admin [name: admin, error_handler: AdminErrors] {
		get /stats Admin.Stats
		get /broken Admin.Broken
	}
}
