// spec: §route-names-and-reverse-urls
package app

scope / {
	get /a A.Show
	get /b B.Other [name: show]
}
