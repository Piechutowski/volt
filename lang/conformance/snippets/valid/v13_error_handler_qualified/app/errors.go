package app

import (
	"net/http"

	volt "github.com/Piechutowski/volt"
)

// Errors is the error_handler the routes name (§V4.4): held like every
// Go reference, with the runtime's ErrorHandler shape spelled exactly.
func Errors(w http.ResponseWriter, r *volt.Request, err error) {}
