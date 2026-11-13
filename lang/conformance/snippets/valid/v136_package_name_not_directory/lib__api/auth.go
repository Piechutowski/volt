package api

import "net/http"

// BearerAuth is the plug routes.volt names (§pipelines).
func BearerAuth(next http.Handler) http.Handler { return next }
