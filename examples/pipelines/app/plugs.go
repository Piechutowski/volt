package app

import "net/http"

// Stamp is a plug of the api pipeline: it adds its name to X-Plugs, so
// the reply lists the plugs it went through, in order (§pipelines).
func Stamp(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Plugs", "Stamp")
		next.ServeHTTP(w, r)
	})
}

// RequireKey is the vault pipeline's plug: without the header
// X-Key: secret it answers 401 and the handler never runs.
func RequireKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Plugs", "RequireKey")
		if r.Header.Get("X-Key") != "secret" {
			http.Error(w, "RequireKey: send X-Key: secret", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
