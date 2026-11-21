package api

import "net/http"

// BearerAuth is the plug routes.volt names (spec/pipelines).
func BearerAuth(next http.Handler) http.Handler { return next }
