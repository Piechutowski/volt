package app

import "net/http"

// BearerAuth is the plug r.volt names (spec/pipelines).
func BearerAuth(next http.Handler) http.Handler { return next }
