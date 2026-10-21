package app

import "net/http"

// BearerAuth and Session are the plugs routes.volt names (§pipelines):
// declared here with the middleware signature, spelled exactly.
func BearerAuth(next http.Handler) http.Handler { return next }
func Session(next http.Handler) http.Handler    { return next }
