// spec: §groups, §selects-over-groups, a name that differs only in case from a group names the right spelling
// want: did you mean the group "series"? names are case-sensitive
package app

table ms_usage {
	id int [pk]
}

group series {
	ms_usage
}

select s for Series
