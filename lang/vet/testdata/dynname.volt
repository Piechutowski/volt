// analyzers: dynname
// The dynamic query layer mints package-scope Go names by concatenation
//: Model+Field for column handles, Model+Limit/Offset/... for
// the option wrappers, and per model the changeset Model+Changeset, the
// aliases Model+Column and Model+Params and the constructor Model+New.
// Distinct declarations can demand the same Go name; generation

// refuses, this rule names both origins.

table users {
  id int [pk]
  foo_bar int
  // handle UserNew collides with users' own changeset constructor UserNew
  new int //WANT dynname
}

// good: a declared type named apart from its column handle; orders.status
// mints the handle OrderStatus, and the type is OrderState
type OrderState string
table orders {
  id int [pk]
  status OrderState
}

// model UserLimit collides with users' generated UserLimit option wrapper
table user_limits { //WANT dynname
  id int [pk]
}

// handle UserFooBar already belongs to users.foo_bar
table user_foos {
  id int [pk]
  bar int //WANT dynname
}

// model UserFooBar (the singular of user_foo_bars) collides with that
// same users.foo_bar handle
table user_foo_bars { //WANT dynname
  id int [pk]
}

// model UserColumn collides with users' column handle type UserColumn
table user_columns { //WANT dynname
  id int [pk]
}

// the derived model UserParam misses users' write body UserParams by a
// letter; the pinned one collides, and a pinned name is checked like a
// derived one
table user_params [model: "UserParams"] { //WANT dynname
  id int [pk]
}

// model UserChangeset, pinned, is users' changeset
table drafts [model: "UserChangeset"] { //WANT dynname
  id int [pk]
}
