// spec: §selects-over-groups, a select's method does not take the name of a method of the Queries handle itself
// want: which is already the Queries method WithCache
package app

Table withs {
	id int [pk]
}

Select cache for withs
