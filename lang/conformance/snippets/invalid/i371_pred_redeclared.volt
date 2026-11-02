// spec: §predicates, §projects-and-packages, Pred names are unique in the package
// want: predicate "busy" redeclared
package app

Table page_views {
	id   int [pk]
	hits int
}

Pred busy { hits > 100 }
Pred busy { hits > 1000 }

Select loud for page_views where busy
