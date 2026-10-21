// spec: §settings
package app

Table posty {
	id int [pk]
}

// singular: makes the member helpers PathPost/PathNewPost/PathEditPost
// while the collection stays PathPosty.
Scope / {
	resources posty [singular: post]
}
