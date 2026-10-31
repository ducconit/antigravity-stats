package web

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// corsMiddleware attaches CORS and Chromium Private Network Access (PNA) headers.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.Header().Set("Access-Control-Allow-Private-Network", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// authMiddleware validates request authentication token if authToken is configured.
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.authToken == "" {
			next(w, r)
			return
		}

		provided := ""
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			provided = strings.TrimPrefix(authHeader, "Bearer ")
		} else if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
			provided = apiKey
		}

		if subtle.ConstantTimeCompare([]byte(provided), []byte(s.authToken)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Unauthorized: Invalid or missing API token"}`))
			return
		}

		next(w, r)
	}
}
