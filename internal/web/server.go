package web

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"antigravity-stats/internal/analyzer"
	"antigravity-stats/internal/web/static"
)

// Server handles HTTP requests and serves the web dashboard and REST APIs.
type Server struct {
	scanner        *analyzer.Scanner
	host           string
	port           int
	authToken      string
	cachedStats    *analyzer.GlobalStats
	cachedSessions []*analyzer.SessionMetric
	cacheMutex     sync.RWMutex
	lastScan       time.Time
}

// ConfigResponse represents the server handshake payload.
type ConfigResponse struct {
	Status       string `json:"status"`
	Version      string `json:"version"`
	AuthRequired bool   `json:"auth_required"`
	DataDir      string `json:"data_dir"`
}

// NewServer initializes a new Server.
func NewServer(scanner *analyzer.Scanner, host string, port int, authToken string) *Server {
	if port <= 0 {
		port = 58763
	}
	return &Server{
		scanner:   scanner,
		host:      host,
		port:      port,
		authToken: authToken,
	}
}

// refreshData refreshes cached scanner data if older than 10 seconds.
func (s *Server) refreshData(force bool) error {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	if !force && time.Since(s.lastScan) < 10*time.Second && s.cachedStats != nil {
		return nil
	}

	stats, sessions, err := s.scanner.ScanAll()
	if err != nil {
		return err
	}

	s.cachedStats = stats
	s.cachedSessions = sessions
	s.lastScan = time.Now()
	return nil
}

// corsMiddleware attaches CORS and Chromium Private Network Access (PNA) headers.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
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

// authMiddleware enforces token authentication when authToken is configured on the server.
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.authToken != "" {
			authHeader := r.Header.Get("Authorization")
			apiKeyHeader := r.Header.Get("X-API-Key")

			token := ""
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			} else if apiKeyHeader != "" {
				token = apiKeyHeader
			}

			if subtle.ConstantTimeCompare([]byte(token), []byte(s.authToken)) != 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error":   "Unauthorized",
					"message": "API key hoặc Token xác thực không chính xác",
				})
				return
			}
		}
		next(w, r)
	}
}

// handleConfig returns server configuration and whether authentication is required.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ConfigResponse{
		Status:       "ok",
		Version:      "1.0.0",
		AuthRequired: s.authToken != "",
		DataDir:      s.scanner.DataDir(),
	})
}

// Handler constructs and returns the HTTP handler with all endpoints and middlewares.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public endpoints
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("GET /", s.handleIndex)

	// Protected endpoints (require auth when authToken is configured)
	mux.HandleFunc("GET /api/stats", s.authMiddleware(s.handleStats))
	mux.HandleFunc("GET /api/sessions", s.authMiddleware(s.handleSessions))
	mux.HandleFunc("GET /api/sessions/{id}", s.authMiddleware(s.handleSessionDetail) )

	return s.corsMiddleware(mux)
}

// Start runs the HTTP server.
func (s *Server) Start() error {
	// Initial scan
	if err := s.refreshData(true); err != nil {
		log.Printf("Warning during initial scan: %v", err)
	}

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	log.Printf("⚡ Antigravity Token Visualizer running on http://%s", addr)
	return http.ListenAndServe(addr, s.Handler())
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	f, err := static.Assets.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	switch {
	case strings.HasSuffix(path, ".html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case strings.HasSuffix(path, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(path, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case strings.HasSuffix(path, ".svg"):
		w.Header().Set("Content-Type", "image/svg+xml")
	case strings.HasSuffix(path, ".png"):
		w.Header().Set("Content-Type", "image/png")
	}
	io.Copy(w, f)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if err := s.refreshData(false); err != nil {
		http.Error(w, fmt.Sprintf("Scan error: %v", err), http.StatusInternalServerError)
		return
	}

	s.cacheMutex.RLock()
	stats := s.cachedStats
	s.cacheMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if err := s.refreshData(false); err != nil {
		http.Error(w, fmt.Sprintf("Scan error: %v", err), http.StatusInternalServerError)
		return
	}

	s.cacheMutex.RLock()
	sessions := s.cachedSessions
	s.cacheMutex.RUnlock()

	// Return lightweight sessions without full steps
	type lightweightSession struct {
		ID           string                    `json:"id"`
		Title        string                    `json:"title"`
		ProjectName  string                    `json:"project_name"`
		CreatedAt    time.Time                 `json:"created_at"`
		LastModified time.Time                 `json:"last_modified"`
		StepCount    int                       `json:"step_count"`
		InputTokens  int64                     `json:"input_tokens"`
		CacheRead    int64                     `json:"cache_read_tokens"`
		OutputTokens int64                     `json:"output_tokens"`
		TotalTokens  int64                     `json:"total_tokens"`
		CacheHitRate float64                   `json:"cache_hit_rate"`
		CostUSD      float64                   `json:"cost_usd"`
		CostVND      float64                   `json:"cost_vnd"`
		ModelName    string                    `json:"model_name"`
		Workspace    string                    `json:"workspace,omitempty"`
		Models       []analyzer.ModelUsageStat `json:"models,omitempty"`
	}

	results := make([]lightweightSession, 0, len(sessions))
	for _, sess := range sessions {
		results = append(results, lightweightSession{
			ID:           sess.ID,
			Title:        sess.Title,
			ProjectName:  sess.ProjectName,
			CreatedAt:    sess.CreatedAt,
			LastModified: sess.LastModified,
			StepCount:    sess.StepCount,
			InputTokens:  sess.InputTokens,
			CacheRead:    sess.CacheReadTokens,
			OutputTokens: sess.OutputTokens,
			TotalTokens:  sess.TotalTokens,
			CacheHitRate: sess.CacheHitRate,
			CostUSD:      sess.CostUSD,
			CostVND:      sess.CostVND,
			ModelName:    sess.ModelName,
			Workspace:    sess.Workspace,
			Models:       sess.Models,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *Server) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") {
		http.Error(w, "Session ID invalid or missing", http.StatusBadRequest)
		return
	}

	session, err := s.scanner.GetSessionDetail(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load session: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}
