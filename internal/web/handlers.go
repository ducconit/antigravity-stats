package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"antigravity-stats/internal/analyzer"
)

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Browser requesting HTML is redirected directly to the GitHub Pages hosted dashboard
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, DefaultDashboardURL, http.StatusTemporaryRedirect)
		return
	}

	// API consumers receive service status JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(RootResponse{
		Service:      "Antigravity Token Visualizer API Server",
		Version:      "1.1.0",
		Status:       "running",
		AuthRequired: s.authToken != "",
		DashboardURL: DefaultDashboardURL,
	})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ConfigResponse{
		Status:       "ok",
		Version:      "1.1.0",
		AuthRequired: s.authToken != "",
		DataDir:      s.scanner.DataDir(),
	})
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

	type SessionListItem struct {
		ID           string                  `json:"id"`
		Title        string                  `json:"title"`
		ProjectName  string                  `json:"project_name"`
		Workspace    string                  `json:"workspace"`
		ModelName    string                  `json:"model_name"`
		Models       []analyzer.ModelUsageStat `json:"models"`
		StepCount    int                     `json:"step_count"`
		TotalTokens  int64                   `json:"total_tokens"`
		InputTokens  int64                   `json:"input_tokens"`
		CacheRead    int64                   `json:"cache_read_tokens"`
		OutputTokens         int64                     `json:"output_tokens"`
		CacheHitRate         float64                   `json:"cache_hit_rate"`
		CostUSD              float64                   `json:"cost_usd"`
		CostVND              float64                   `json:"cost_vnd"`
		CreatedAt            string                    `json:"created_at"`
		LastModified         string                    `json:"last_modified"`
		AgentName            string                    `json:"agent_name"`
		ParentConversationID string                    `json:"parent_conversation_id"`
		NestingDepth         int                       `json:"nesting_depth"`
		IsSubagent           bool                      `json:"is_subagent"`
		ClientTool           string                    `json:"client_tool,omitempty"`
	}

	list := make([]SessionListItem, 0, len(sessions))
	for _, s := range sessions {
		list = append(list, SessionListItem{
			ID:                   s.ID,
			Title:                s.Title,
			ProjectName:          s.ProjectName,
			Workspace:            s.Workspace,
			ModelName:            s.ModelName,
			Models:               s.Models,
			StepCount:            s.StepCount,
			TotalTokens:          s.TotalTokens,
			InputTokens:          s.InputTokens,
			CacheRead:            s.CacheReadTokens,
			OutputTokens:         s.OutputTokens,
			CacheHitRate:         s.CacheHitRate,
			CostUSD:              s.CostUSD,
			CostVND:              s.CostVND,
			CreatedAt:            s.CreatedAt.Format("2006-01-02 15:04:05"),
			LastModified:         s.LastModified.Format("2006-01-02 15:04:05"),
			AgentName:            s.AgentName,
			ParentConversationID: s.ParentConversationID,
			NestingDepth:         s.NestingDepth,
			IsSubagent:           s.IsSubagent,
			ClientTool:           s.ClientTool,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (s *Server) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") {
		http.Error(w, "Session ID invalid or missing", http.StatusBadRequest)
		return
	}

	session, err := s.scanner.GetSessionDetail(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Session not found: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}
