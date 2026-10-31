package web

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"antigravity-stats/internal/analyzer"
)

// DefaultDashboardURL is the official hosted dashboard on GitHub Pages.
const DefaultDashboardURL = "https://ducconit.github.io/antigravity-stats/"

// Server handles HTTP requests and serves REST APIs.
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
	DataDir      string `json:"data_dir,omitempty"`
}

// RootResponse represents the root service info.
type RootResponse struct {
	Service      string `json:"service"`
	Version      string `json:"version"`
	Status       string `json:"status"`
	AuthRequired bool   `json:"auth_required"`
	DashboardURL string `json:"dashboard_url"`
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

// Handler constructs and returns the HTTP handler with all endpoints and middlewares.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public handshake endpoint
	mux.HandleFunc("GET /api/config", s.handleConfig)

	// Root endpoint (browser HTML redirect or API service status)
	mux.HandleFunc("GET /", s.handleIndex)

	// API endpoints
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("GET /api/sessions/{id}", s.handleSessionDetail)

	// Wrap mux with authMiddleware (protects all endpoints except /api/config and HTML redirect at /),
	// then with corsMiddleware (attaches CORS headers and handles preflight OPTIONS).
	return s.corsMiddleware(s.authMiddleware(mux))
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
