package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"antigravity-stats/internal/analyzer"
)

func setupTestEnvironment(t *testing.T) (*analyzer.Scanner, string) {
	tempDir := t.TempDir()
	sessionID := "test-session-4567"
	logDir := filepath.Join(tempDir, "brain", sessionID, ".system_generated", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("failed to create log dir: %v", err)
	}

	logPath := filepath.Join(logDir, "transcript.jsonl")
	sample := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-31T12:00:00Z","content":"<USER_REQUEST>\nBuild visualizer\n</USER_REQUEST>"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-31T12:00:02Z","input_tokens":500,"cache_read_tokens":1500,"output_tokens":100,"thinking":"Working on it...","tool_calls":[{"name":"run_command","args":{"toolSummary":"Check files"}}]}
`
	if err := os.WriteFile(logPath, []byte(sample), 0644); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	scanner := analyzer.NewScanner(tempDir)
	return scanner, sessionID
}

func TestServerEndpoints(t *testing.T) {
	scanner, validSessionID := setupTestEnvironment(t)
	server := NewServer(scanner, "127.0.0.1", 0, "")

	// Refresh cache
	if err := server.refreshData(true); err != nil {
		t.Fatalf("refreshData failed: %v", err)
	}

	handler := server.Handler()

	// 1. Test GET /
	t.Run("GET / with HTML Accept redirects to dashboard", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusTemporaryRedirect {
			t.Errorf("expected 307 Temporary Redirect, got %d", w.Code)
		}
		loc := w.Header().Get("Location")
		if loc != DefaultDashboardURL {
			t.Errorf("expected redirect to %s, got %s", DefaultDashboardURL, loc)
		}
	})

	t.Run("GET / with JSON returns service info", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
		var root RootResponse
		if err := json.Unmarshal(w.Body.Bytes(), &root); err != nil {
			t.Fatalf("failed to decode root JSON: %v", err)
		}
		if root.Status != "running" || root.DashboardURL != DefaultDashboardURL {
			t.Errorf("unexpected root payload: %+v", root)
		}
	})

	t.Run("GET /invalid returns 404", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/invalid-path", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	// 2. Test GET /api/stats
	t.Run("GET /api/stats", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/stats", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
		var stats analyzer.GlobalStats
		if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
			t.Fatalf("failed to unmarshal stats JSON: %v", err)
		}
		if stats.TotalSessions != 1 {
			t.Errorf("expected 1 session, got %d", stats.TotalSessions)
		}
		if stats.TotalInputTokens != 500 {
			t.Errorf("expected 500 input tokens, got %d", stats.TotalInputTokens)
		}
		if stats.TotalCacheRead != 1500 {
			t.Errorf("expected 1500 cache read tokens, got %d", stats.TotalCacheRead)
		}
	})

	// 3. Test GET /api/sessions
	t.Run("GET /api/sessions", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/sessions", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
		var sessions []map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &sessions); err != nil {
			t.Fatalf("failed to unmarshal sessions JSON: %v", err)
		}
		if len(sessions) != 1 {
			t.Errorf("expected 1 session in list, got %d", len(sessions))
		}
	})

	// 4. Test GET /api/sessions/{id} with valid ID
	t.Run("GET /api/sessions/{id} valid", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/sessions/"+validSessionID, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
		var detail analyzer.SessionMetric
		if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
			t.Fatalf("failed to unmarshal session detail: %v", err)
		}
		if detail.ID != validSessionID {
			t.Errorf("expected ID %s, got %s", validSessionID, detail.ID)
		}
		if len(detail.Steps) != 2 {
			t.Errorf("expected 2 steps, got %d", len(detail.Steps))
		}
	})

	// 5. Test GET /api/sessions/{id} with non-existent ID
	t.Run("GET /api/sessions/{id} not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/sessions/non-existent-uuid", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})

	// 6. Test GET /api/sessions/{id} with invalid characters / directory traversal
	t.Run("GET /api/sessions/{id} invalid traversal", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/sessions/..%2F..%2Fetc", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

func TestServerConfigAndCORS(t *testing.T) {
	scanner, _ := setupTestEnvironment(t)
	server := NewServer(scanner, "127.0.0.1", 0, "")
	handler := server.Handler()

	// 1. Handshake config endpoint
	t.Run("GET /api/config", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/config", nil)
		req.Header.Set("Origin", "https://ducconit.github.io")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		// Verify CORS headers
		if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != "https://ducconit.github.io" {
			t.Errorf("expected Access-Control-Allow-Origin reflection, got %s", origin)
		}
		if pna := w.Header().Get("Access-Control-Allow-Private-Network"); pna != "true" {
			t.Errorf("expected Access-Control-Allow-Private-Network: true, got %s", pna)
		}

		var cfg ConfigResponse
		if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
			t.Fatalf("failed to decode config json: %v", err)
		}
		if cfg.Status != "ok" || cfg.AuthRequired != false || cfg.Version != "1.1.0" {
			t.Errorf("unexpected config response: %+v", cfg)
		}
	})

	// 2. Preflight OPTIONS request
	t.Run("OPTIONS /api/stats Preflight", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/api/stats", nil)
		req.Header.Set("Origin", "https://ducconit.github.io")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204 No Content for OPTIONS, got %d", w.Code)
		}
		if pna := w.Header().Get("Access-Control-Allow-Private-Network"); pna != "true" {
			t.Errorf("expected PNA header on OPTIONS, got %s", pna)
		}
		if methods := w.Header().Get("Access-Control-Allow-Methods"); methods == "" {
			t.Errorf("missing Access-Control-Allow-Methods")
		}
	})
}

func TestServerAuthentication(t *testing.T) {
	scanner, _ := setupTestEnvironment(t)
	expectedToken := "my-secret-key-123"
	server := NewServer(scanner, "127.0.0.1", 0, expectedToken)
	if err := server.refreshData(true); err != nil {
		t.Fatalf("refreshData failed: %v", err)
	}
	handler := server.Handler()

	// 1. Config should indicate auth_required: true
	t.Run("Config reports AuthRequired true", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/config", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}
		var cfg ConfigResponse
		if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if !cfg.AuthRequired {
			t.Errorf("expected AuthRequired to be true")
		}
	})

	// 2. Request without token should fail with 401
	t.Run("GET /api/stats without token returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/stats", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	// 3. Request with wrong token should fail with 401
	t.Run("GET /api/stats with invalid token returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/stats", nil)
		req.Header.Set("Authorization", "Bearer wrong-secret")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	// 4. Request with valid Bearer token should succeed
	t.Run("GET /api/stats with valid Bearer token returns 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/stats", nil)
		req.Header.Set("Authorization", "Bearer "+expectedToken)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	// 5. Request with valid X-API-Key header should succeed
	t.Run("GET /api/stats with valid X-API-Key returns 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/stats", nil)
		req.Header.Set("X-API-Key", expectedToken)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	// 6. Request with raw Authorization header (without Bearer) should succeed
	t.Run("GET /api/stats with raw Authorization header returns 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/stats", nil)
		req.Header.Set("Authorization", expectedToken)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	// 7. Request with ?api_key= query parameter should succeed
	t.Run("GET /api/stats with ?api_key= query param returns 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/stats?api_key="+expectedToken, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	// 8. Request with ?token= query parameter should succeed
	t.Run("GET /api/stats with ?token= query param returns 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/stats?token="+expectedToken, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	// 9. GET /api/sessions without token should fail with 401
	t.Run("GET /api/sessions without token returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/sessions", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	// 10. GET /api/sessions with valid token should succeed
	t.Run("GET /api/sessions with valid token returns 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/sessions", nil)
		req.Header.Set("X-API-Key", expectedToken)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	// 11. GET /api/sessions/{id} without token should fail with 401
	t.Run("GET /api/sessions/{id} without token returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/sessions/test-session-4567", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	// 12. GET /api/sessions/{id} with valid token should succeed
	t.Run("GET /api/sessions/{id} with valid token returns 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/sessions/test-session-4567", nil)
		req.Header.Set("X-API-Key", expectedToken)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	// 13. Root endpoint JSON request without token should fail with 401
	t.Run("GET / without token (JSON) returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	// 14. Root endpoint JSON request with valid token should succeed
	t.Run("GET / with valid token (JSON) returns 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-API-Key", expectedToken)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
	})

	// 15. Root endpoint HTML request should redirect to dashboard without token (public browser entry)
	t.Run("GET / with HTML Accept redirects to dashboard without token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusTemporaryRedirect {
			t.Errorf("expected 307 Temporary Redirect, got %d", w.Code)
		}
	})
}

// TestServerWithStaticFS verifies that embedded/local Web UI assets are served directly
// and SPA client routes fallback to index.html without redirects.
func TestServerWithStaticFS(t *testing.T) {
	scanner, _ := setupTestEnvironment(t)
	server := NewServer(scanner, "127.0.0.1", 0, "")

	// Create a dummy filesystem with an index.html and test asset
	dummyFS := http.Dir(t.TempDir())
	indexPath := filepath.Join(string(dummyFS), "index.html")
	if err := os.WriteFile(indexPath, []byte("<!DOCTYPE html><html><body>Dashboard</body></html>"), 0644); err != nil {
		t.Fatalf("failed to write dummy index.html: %v", err)
	}

	server.SetStaticFS(dummyFS)
	if !server.HasStaticFS() {
		t.Errorf("expected HasStaticFS to be true")
	}

	handler := server.Handler()

	t.Run("GET / with HTML Accept serves index.html (200 OK)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", "text/html")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "Dashboard") {
			t.Errorf("expected body to contain 'Dashboard', got %s", body)
		}
	})

	t.Run("GET /login with HTML Accept falls back to index.html (SPA routing)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/login", nil)
		req.Header.Set("Accept", "text/html")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "Dashboard") {
			t.Errorf("expected body to contain 'Dashboard', got %s", body)
		}
	})
}
