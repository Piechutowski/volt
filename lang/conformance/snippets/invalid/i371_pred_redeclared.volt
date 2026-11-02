// spec: §predicates, §projects-and-packages, Pred names are unique in the package
// want: predicate "busy" redeclared
package app

table page_views {
	id   int [pk]
	hits int
}

pred busy { hits > 100 }
pred busy { hits > 1000 }

select loud for page_views where busy
