package app

import (
	"net/http"

	volt "github.com/Piechutowski/volt"
)

// BearerAuth is the plug routes.volt names (§pipelines). Go ignores this
// testdata directory; the Volt checker reads it.
func BearerAuth(next http.Handler) http.Handler { return next }

// Errors is the error_handler routes.volt names (§scopes).
func Errors(w http.ResponseWriter, r *volt.Request, err error) {}
