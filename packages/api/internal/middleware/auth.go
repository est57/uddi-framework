package middleware

import (
	"net/http"

	"github.com/uddi-protocol/uddi/api/internal/response"
)

func RequireAPIKey(store APIKeyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			serviceID := r.Header.Get("X-Service-ID")
			if apiKey == "" {
				response.Error(w, http.StatusUnauthorized, "missing API key")
				return
			}
			if serviceID == "" {
				response.Error(w, http.StatusUnauthorized, "missing service ID")
				return
			}
			if err := store.Validate(r.Context(), serviceID, apiKey); err != nil {
				response.Error(w, http.StatusUnauthorized, "invalid API key")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireAPIKeyPresence(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") == "" {
			response.Error(w, http.StatusUnauthorized, "missing API key")
			return
		}
		next.ServeHTTP(w, r)
	})
}
