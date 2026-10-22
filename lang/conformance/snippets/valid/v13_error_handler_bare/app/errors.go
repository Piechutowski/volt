package app

import (
	"net/http"

	volt "github.com/Piechutowski/volt"
)

// Errors is the error_handler the routes name (§scopes): held like every
// Go reference, with the runtime's ErrorHandler shape spelled exactly.
func Errors(w http.ResponseWriter, r *volt.Request, err error) {}
