// spec: §settings-lists, §dataset-settings, a dataset takes each setting at most once
// want: setting "strip" may appear at most once
package app

table ms_revenue {
	id int [pk]
}

select rows for ms_revenue

scope /ms {
	dataset rows [strip: "ms_", strip: "ms"]
}
