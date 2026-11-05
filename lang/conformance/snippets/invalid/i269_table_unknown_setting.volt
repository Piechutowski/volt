// spec: §table, model is the only table setting: headercolor is unknown whatever its value
// want: unknown setting "headercolor"
table users [headercolor: "red"] {
  id int [pk]
}
