package app

import "net/http"

// BearerAuth is the plug r.volt names (§pipelines).
func BearerAuth(next http.Handler) http.Handler { return next }
