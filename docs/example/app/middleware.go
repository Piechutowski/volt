// YOURS — referenced by name from routes.volt (`use app.BearerAuth`).
// The ecosystem contract, nothing Volt-specific: any chi/otelhttp/CORS
// middleware drops into a Pipeline exactly the same way (R8).
package app

import "net/http"

func BearerAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !validToken(r.Header.Get("Authorization")) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func validToken(h string) bool { /* … */ return h != "" }
