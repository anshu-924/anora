package middleware

import (
	"encoding/json"
	"net/http"
	"os"
)

// AuthMiddleware validates the X-API-KEY header
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-KEY")
		expectedKey := os.Getenv("X_API_KEY")

		if expectedKey == "" {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Server configuration error"})
			return
		}

		if apiKey == "" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Missing X-API-KEY header"})
			return
		}

		if apiKey != expectedKey {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid API key"})
			return
		}

		// API key is valid, proceed to the next handler
		next(w, r)
	}
}
