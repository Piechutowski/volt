package app

// LoginForm is a struct body (§request-bodies).
type LoginForm struct {
	Email    string
	Password string
}

// Clock is not a struct; a body cannot decode into it.
type Clock int
