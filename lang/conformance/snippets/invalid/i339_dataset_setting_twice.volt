// spec: §settings-lists, §dataset-settings, a dataset takes each setting at most once
// want: setting "strip" may appear at most once
package app

Table ms_revenue {
	id int [pk]
}

Select rows for ms_revenue

Scope /ms {
	dataset rows [strip: "ms_", strip: "ms"]
}
