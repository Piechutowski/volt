// spec: §settings
package app

table posty {
	id int [pk]
}

// singular: makes the member helpers PathPost/PathNewPost/PathEditPost
// while the collection stays PathPosty.
scope / {
	resources posty [singular: post]
}
