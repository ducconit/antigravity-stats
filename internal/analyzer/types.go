package analyzer

import "time"

// StepMetric represents token and performance metrics for a single step in a conversation.
type StepMetric struct {
	StepIndex       int            `json:"step_index"`
	Source          string         `json:"source"`
	Type            string         `json:"type"`
	Status          string         `json:"status"`
	ModelName       string         `json:"model_name"`
	CreatedAt       time.Time      `json:"created_at"`
	InputTokens     int64          `json:"input_tokens"`
	CacheReadTokens int64          `json:"cache_read_tokens"`
	OutputTokens    int64          `json:"output_tokens"`
	TotalStepTokens int64          `json:"total_step_tokens"`
	CacheHitRate    float64        `json:"cache_hit_rate"`
	ToolCalls       []ToolCallInfo `json:"tool_calls,omitempty"`
	ThinkingPreview string         `json:"thinking_preview,omitempty"`
	ContentPreview  string         `json:"content_preview,omitempty"`
}

// ToolCallInfo holds brief info about a tool invocation.
type ToolCallInfo struct {
	Name        string `json:"name"`
	ToolSummary string `json:"tool_summary,omitempty"`
}

// ModelUsageStat represents token and cost metrics grouped by model.
type ModelUsageStat struct {
	ModelName           string  `json:"model_name"`
	StepCount           int     `json:"step_count"`
	InputTokens         int64   `json:"input_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	OverallCacheHitRate float64 `json:"overall_cache_hit_rate"`
	CostUSD             float64 `json:"cost_usd"`
	CostVND             float64 `json:"cost_vnd"`
}

// SessionMetric represents aggregated metrics for a full conversation session.
type SessionMetric struct {
	ID              string           `json:"id"`
	Title           string           `json:"title"`
	ProjectName     string           `json:"project_name"`
	Workspace       string           `json:"workspace,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	LastModified    time.Time        `json:"last_modified"`
	StepCount       int              `json:"step_count"`
	InputTokens     int64            `json:"input_tokens"`
	CacheReadTokens int64            `json:"cache_read_tokens"`
	OutputTokens    int64            `json:"output_tokens"`
	TotalTokens     int64            `json:"total_tokens"`
	CacheHitRate    float64          `json:"cache_hit_rate"` // CacheRead / (CacheRead + Input) * 100
	CostUSD         float64          `json:"cost_usd"`
	CostVND         float64          `json:"cost_vnd"`
	ModelName       string           `json:"model_name"` // primary or first model
	Models          []ModelUsageStat `json:"models"`     // grouped by model
	Steps           []StepMetric     `json:"steps,omitempty"`
}

// ProjectStats aggregates metrics for a specific project.
type ProjectStats struct {
	Name                string           `json:"name"`
	WorkspacePath       string           `json:"workspace_path,omitempty"`
	SessionCount        int              `json:"session_count"`
	ActiveSessionCount  int              `json:"active_session_count"`
	TotalTokens         int64            `json:"total_tokens"`
	TotalInputTokens    int64            `json:"total_input_tokens"`
	TotalCacheRead      int64            `json:"total_cache_read"`
	TotalOutput         int64            `json:"total_output"`
	OverallCacheHitRate float64          `json:"overall_cache_hit_rate"`
	TotalCostUSD        float64          `json:"total_cost_usd"`
	TotalCostVND        float64          `json:"total_cost_vnd"`
	LastModified        time.Time        `json:"last_modified"`
	Models              []ModelUsageStat `json:"models"` // grouped by model across project
	Sessions            []*SessionMetric `json:"sessions,omitempty"`
}

// DailyMetric summarizes token metrics for a single day.
type DailyMetric struct {
	Date            string  `json:"date"` // YYYY-MM-DD
	SessionCount    int     `json:"session_count"`
	InputTokens     int64   `json:"input_tokens"`
	CacheReadTokens int64   `json:"cache_read_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	TotalTokens     int64   `json:"total_tokens"`
	CacheHitRate    float64 `json:"cache_hit_rate"`
	CostUSD         float64 `json:"cost_usd"`
	CostVND         float64 `json:"cost_vnd"`
}

// GlobalStats contains aggregate metrics across all analyzed sessions.
type GlobalStats struct {
	TotalSessions       int              `json:"total_sessions"`
	ActiveTokenSessions int              `json:"active_token_sessions"`
	TotalSteps          int              `json:"total_steps"`
	TotalInputTokens    int64            `json:"total_input_tokens"`
	TotalCacheRead      int64            `json:"total_cache_read"`
	TotalOutput         int64            `json:"total_output"`
	TotalTokens         int64            `json:"total_tokens"`
	OverallCacheHitRate float64          `json:"overall_cache_hit_rate"`
	TotalCostUSD        float64          `json:"total_cost_usd"`
	TotalCostVND        float64          `json:"total_cost_vnd"`
	DailyMetrics        []DailyMetric    `json:"daily_metrics"`
	ModelDistribution   map[string]int64 `json:"model_distribution"`
	Projects            []ProjectStats   `json:"projects"`
	RecentSessions      []SessionMetric  `json:"recent_sessions"`
}
