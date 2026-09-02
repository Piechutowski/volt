package app

import "net/http"

func BasicAuth(next http.Handler) http.Handler { return next }
