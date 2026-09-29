package auth

import (
	"net/http"
	"strings"
)

// RequireBearer rejects requests when requiredKey is non-empty and Authorization does not match.
func RequireBearer(requiredKey string, next http.Handler) http.Handler {
	if requiredKey == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !bearerMatches(r, requiredKey) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireBearerFunc is the HandlerFunc variant of RequireBearer.
func RequireBearerFunc(requiredKey string, next http.HandlerFunc) http.HandlerFunc {
	return RequireBearer(requiredKey, next).ServeHTTP
}

func bearerMatches(r *http.Request, key string) bool {
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return false
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
	return token == key
}
