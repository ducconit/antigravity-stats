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

// extractToken retrieves the authentication token/API key from standard locations:
// 1. Authorization header: "Bearer <token>" or raw "<token>"
// 2. X-API-Key header: "<token>"
// 3. Query string: ?token=<token> or ?api_key=<token>
func (s *Server) extractToken(r *http.Request) string {
	// 1. Check Authorization header
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		}
		return strings.TrimSpace(authHeader)
	}

	// 2. Check X-API-Key header
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return strings.TrimSpace(apiKey)
	}

	// 3. Check query parameters (?token=... or ?api_key=...)
	if qTok := r.URL.Query().Get("token"); qTok != "" {
		return strings.TrimSpace(qTok)
	}
	if qKey := r.URL.Query().Get("api_key"); qKey != "" {
		return strings.TrimSpace(qKey)
	}

	return ""
}

// authMiddleware enforces API key/token authentication across all API endpoints except /api/config.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If server does not have an authToken configured, permit all requests
		if s.authToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Whitelist 1: /api/config is strictly public so frontend can perform handshake
		if r.URL.Path == "/api/config" {
			next.ServeHTTP(w, r)
			return
		}

		// Whitelist 2: HTML requests to root "/" redirect to the hosted dashboard
		if r.URL.Path == "/" && strings.Contains(r.Header.Get("Accept"), "text/html") {
			next.ServeHTTP(w, r)
			return
		}

		// All other endpoints (API routes, root JSON, etc.) require a valid API key / token
		provided := s.extractToken(r)

		if subtle.ConstantTimeCompare([]byte(provided), []byte(s.authToken)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Unauthorized: Invalid or missing API key","status":401}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
