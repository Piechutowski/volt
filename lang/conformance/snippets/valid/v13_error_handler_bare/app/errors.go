package app

import (
	volt "github.com/Piechutowski/volt"
)

// Errors is the error_handler the routes name (§scopes): held like every
// Go reference, with the runtime's ErrorHandler shape spelled exactly.
func Errors(r volt.Request, err error) volt.Response { return volt.DefaultErrorHandler(r, err) }
