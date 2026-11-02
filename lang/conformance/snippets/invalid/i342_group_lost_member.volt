// spec: §line-structure-and-whitespace, §groups, a Group body that lost its only member to a syntax error is not reported empty
// want: expected identifier in group member
package app

Table posts {
	id int [pk]
}

Group g {
	"posts"
}
