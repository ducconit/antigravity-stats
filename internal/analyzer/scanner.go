package analyzer

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Scanner reads Antigravity conversation logs and extracts token metrics.
type Scanner struct {
	dataDir string
}

// NewScanner creates a new Scanner instance with resolved data directory.
func NewScanner(dataDir string) *Scanner {
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			dataDir = filepath.Join(home, ".gemini", "antigravity")
		}
	} else if strings.HasPrefix(dataDir, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			dataDir = filepath.Join(home, dataDir[2:])
		}
	}
	return &Scanner{dataDir: dataDir}
}

// DataDir returns the resolved directory path.
func (s *Scanner) DataDir() string {
	return s.dataDir
}

type conversationMeta struct {
	Title         string
	Preview       string
	WorkspaceURIs string
	Project       string
	LastModified  time.Time
}

// loadSummariesFromSQLite loads titles and metadata from conversation_summaries.db.
func (s *Scanner) loadSummariesFromSQLite() map[string]conversationMeta {
	metaMap := make(map[string]conversationMeta)
	dbPath := filepath.Join(s.dataDir, "conversation_summaries.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return metaMap
	}

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return metaMap
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT c.conversation_id, c.title, c.preview, c.workspace_uris, c.last_modified_time, COALESCE(f.project, '')
		FROM conversation_summaries c
		LEFT JOIN (SELECT DISTINCT conversation_id, project FROM conversations_fts WHERE project != '') f
		ON c.conversation_id = f.conversation_id
	`)
	if err != nil {
		rows, err = db.Query("SELECT conversation_id, title, preview, workspace_uris, last_modified_time, '' FROM conversation_summaries")
		if err != nil {
			return metaMap
		}
	}
	defer rows.Close()

	for rows.Next() {
		var id, title, preview, workspaceURIs, lastModStr, proj string
		if err := rows.Scan(&id, &title, &preview, &workspaceURIs, &lastModStr, &proj); err == nil {
			normalized := strings.Replace(lastModStr, " ", "T", 1)
			t, _ := time.Parse(time.RFC3339, normalized)
			metaMap[id] = conversationMeta{
				Title:         title,
				Preview:       preview,
				WorkspaceURIs: workspaceURIs,
				Project:       proj,
				LastModified:  t,
			}
		}
	}
	return metaMap
}

type rawStep struct {
	StepIndex       int    `json:"step_index"`
	Source          string `json:"source"`
	Type            string `json:"type"`
	Status          string `json:"status"`
	CreatedAt       string `json:"created_at"`
	Content         string `json:"content"`
	Thinking        string `json:"thinking"`
	InputTokens     *int64 `json:"input_tokens"`
	CacheReadTokens *int64 `json:"cache_read_tokens"`
	OutputTokens    *int64 `json:"output_tokens"`
	ToolCalls       []struct {
		Name string                 `json:"name"`
		Args map[string]interface{} `json:"args"`
	} `json:"tool_calls"`
}

var (
	userRequestRegex = regexp.MustCompile(`(?s)<USER_REQUEST>(.*?)</USER_REQUEST>`)
	modelSelectRegex = regexp.MustCompile(`(?:Model Selection` + "`" + ` from .*? to|from .*? to) (.*?)\.`)
)

func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

// extractProjectName extracts a clean project name from ftsProject or workspaceURIs JSON string.
func extractProjectName(ftsProject, workspaceURIs string) (projectName string, workspacePath string) {
	if ftsProject != "" {
		return ftsProject, ""
	}
	if workspaceURIs == "" || workspaceURIs == "[]" {
		return "Khác / Global", ""
	}
	var uris []string
	if err := json.Unmarshal([]byte(workspaceURIs), &uris); err == nil && len(uris) > 0 {
		first := uris[0]
		trimmed := strings.TrimPrefix(first, "file://")
		workspacePath = trimmed
		if strings.Contains(trimmed, ".system_generated") {
			return "Subagent Worktree", workspacePath
		}
		base := filepath.Base(trimmed)
		if base != "" && base != "." && base != "/" {
			return base, workspacePath
		}
	}
	return "Khác / Global", workspacePath
}

// ParseTranscriptFile parses a single transcript.jsonl file into a SessionMetric.
func ParseTranscriptFile(sessionID, filePath string, meta conversationMeta) (*SessionMetric, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	projectName, wsPath := extractProjectName(meta.Project, meta.WorkspaceURIs)
	if wsPath == "" && meta.WorkspaceURIs != "" && meta.WorkspaceURIs != "[]" {
		wsPath = meta.WorkspaceURIs
	}

	session := &SessionMetric{
		ID:          sessionID,
		Title:       meta.Title,
		ProjectName: projectName,
		Workspace:   wsPath,
		ModelName:   "Gemini 3.8 Flash",
		Models:      make([]ModelUsageStat, 0),
		Steps:       make([]StepMetric, 0),
	}

	currentModel := "Gemini 3.8 Flash"
	modelStatsMap := make(map[string]*ModelUsageStat)

	reader := bufio.NewReader(file)
	var firstCreatedAt time.Time
	var lastCreatedAt time.Time

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			var raw rawStep
			if jsonErr := json.Unmarshal(line, &raw); jsonErr == nil {
				parsedTime, _ := time.Parse(time.RFC3339, raw.CreatedAt)
				if firstCreatedAt.IsZero() && !parsedTime.IsZero() {
					firstCreatedAt = parsedTime
				}
				if !parsedTime.IsZero() {
					lastCreatedAt = parsedTime
				}

				// Check for model selection / change
				if raw.Content != "" {
					if match := modelSelectRegex.FindStringSubmatch(raw.Content); len(match) > 1 {
						currentModel = strings.TrimSpace(match[1])
						if session.ModelName == "Gemini 3.8 Flash" || session.ModelName == "" {
							session.ModelName = currentModel
						}
					}
				}

				// Extract Title from step 0 if not present in SQLite
				if raw.StepIndex == 0 {
					if session.Title == "" && raw.Content != "" {
						if match := userRequestRegex.FindStringSubmatch(raw.Content); len(match) > 1 {
							trimmed := strings.TrimSpace(match[1])
							lines := strings.Split(trimmed, "\n")
							if len(lines) > 0 && lines[0] != "" {
								session.Title = truncateRunes(lines[0], 100)
							}
						}
					}
				}

				var inTok, cacheTok, outTok int64
				if raw.InputTokens != nil {
					inTok = *raw.InputTokens
					session.InputTokens += inTok
				}
				if raw.CacheReadTokens != nil {
					cacheTok = *raw.CacheReadTokens
					session.CacheReadTokens += cacheTok
				}
				if raw.OutputTokens != nil {
					outTok = *raw.OutputTokens
					session.OutputTokens += outTok
				}

				stepTotal := inTok + cacheTok + outTok
				stepCacheRate := 0.0
				if cacheTok+inTok > 0 {
					stepCacheRate = (float64(cacheTok) / float64(cacheTok+inTok)) * 100.0
				}

				// Accumulate into modelStatsMap
				if stepTotal > 0 || raw.Type == "PLANNER_RESPONSE" {
					mStat, exists := modelStatsMap[currentModel]
					if !exists {
						mStat = &ModelUsageStat{ModelName: currentModel}
						modelStatsMap[currentModel] = mStat
					}
					mStat.StepCount++
					mStat.InputTokens += inTok
					mStat.CacheReadTokens += cacheTok
					mStat.OutputTokens += outTok
					mStat.TotalTokens += stepTotal
				}

				var tools []ToolCallInfo
				for _, tc := range raw.ToolCalls {
					summary := ""
					if s, ok := tc.Args["toolSummary"].(string); ok {
						summary = strings.Trim(s, "\"")
					}
					tools = append(tools, ToolCallInfo{
						Name:        tc.Name,
						ToolSummary: summary,
					})
				}

				thinkingPreview := ""
				if len(raw.Thinking) > 0 {
					thinkingPreview = truncateRunes(raw.Thinking, 200)
				}

				contentPreview := ""
				if len(raw.Content) > 0 {
					contentPreview = truncateRunes(raw.Content, 200)
				}

				session.Steps = append(session.Steps, StepMetric{
					StepIndex:       raw.StepIndex,
					Source:          raw.Source,
					Type:            raw.Type,
					Status:          raw.Status,
					ModelName:       currentModel,
					CreatedAt:       parsedTime,
					InputTokens:     inTok,
					CacheReadTokens: cacheTok,
					OutputTokens:    outTok,
					TotalStepTokens: stepTotal,
					CacheHitRate:    stepCacheRate,
					ToolCalls:       tools,
					ThinkingPreview: thinkingPreview,
					ContentPreview:  contentPreview,
				})
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			break
		}
	}

	session.StepCount = len(session.Steps)
	session.CreatedAt = firstCreatedAt
	session.LastModified = lastCreatedAt
	if session.LastModified.IsZero() && !meta.LastModified.IsZero() {
		session.LastModified = meta.LastModified
	}
	if session.Title == "" {
		if len(sessionID) > 8 {
			session.Title = "Conversation " + sessionID[:8]
		} else {
			session.Title = "Conversation " + sessionID
		}
	}

	// Compile session.Models
	for _, mStat := range modelStatsMap {
		if mStat.CacheReadTokens+mStat.InputTokens > 0 {
			mStat.OverallCacheHitRate = (float64(mStat.CacheReadTokens) / float64(mStat.CacheReadTokens+mStat.InputTokens)) * 100.0
		}
		mStat.CostUSD, mStat.CostVND = CalculateCost(mStat.ModelName, mStat.InputTokens, mStat.CacheReadTokens, mStat.OutputTokens)
		session.Models = append(session.Models, *mStat)
	}
	sort.Slice(session.Models, func(i, j int) bool {
		return session.Models[i].TotalTokens > session.Models[j].TotalTokens
	})

	session.TotalTokens = session.InputTokens + session.CacheReadTokens + session.OutputTokens
	if session.CacheReadTokens+session.InputTokens > 0 {
		session.CacheHitRate = (float64(session.CacheReadTokens) / float64(session.CacheReadTokens+session.InputTokens)) * 100.0
	}
	session.CostUSD, session.CostVND = CalculateCost(session.ModelName, session.InputTokens, session.CacheReadTokens, session.OutputTokens)

	return session, nil
}

// ScanAll scans all conversations in the data directory and compiles aggregate statistics.
func (s *Scanner) ScanAll() (*GlobalStats, []*SessionMetric, error) {
	metaMap := s.loadSummariesFromSQLite()

	brainDir := filepath.Join(s.dataDir, "brain")
	entries, err := os.ReadDir(brainDir)
	if err != nil {
		return nil, nil, err
	}

	type scanJob struct {
		id       string
		path     string
		metaInfo conversationMeta
	}

	var jobs []scanJob
	for _, entry := range entries {
		if entry.IsDir() {
			sessID := entry.Name()
			transcriptPath := filepath.Join(brainDir, sessID, ".system_generated", "logs", "transcript.jsonl")
			if _, err := os.Stat(transcriptPath); err == nil {
				jobs = append(jobs, scanJob{
					id:       sessID,
					path:     transcriptPath,
					metaInfo: metaMap[sessID],
				})
			}
		}
	}

	numWorkers := 8
	jobsChan := make(chan scanJob, len(jobs))
	resultsChan := make(chan *SessionMetric, len(jobs))

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobsChan {
				session, err := ParseTranscriptFile(job.id, job.path, job.metaInfo)
				if err == nil && session != nil {
					resultsChan <- session
				}
			}
		}()
	}

	for _, job := range jobs {
		jobsChan <- job
	}
	close(jobsChan)

	wg.Wait()
	close(resultsChan)

	var allSessions []*SessionMetric
	stats := &GlobalStats{
		TotalSessions:     len(jobs),
		ModelDistribution: make(map[string]int64),
		Projects:          make([]ProjectStats, 0),
	}

	dailyMap := make(map[string]*DailyMetric)
	projectMap := make(map[string]*ProjectStats)
	projModelMap := make(map[string]map[string]*ModelUsageStat)

	for session := range resultsChan {
		allSessions = append(allSessions, session)
		stats.TotalSteps += session.StepCount

		// Project grouping
		projName := session.ProjectName
		if projName == "" {
			projName = "Khác / Global"
		}
		proj, exists := projectMap[projName]
		if !exists {
			proj = &ProjectStats{
				Name:          projName,
				WorkspacePath: session.Workspace,
				Models:        make([]ModelUsageStat, 0),
				Sessions:      make([]*SessionMetric, 0),
			}
			projectMap[projName] = proj
		}
		proj.SessionCount++
		if session.LastModified.After(proj.LastModified) {
			proj.LastModified = session.LastModified
		}
		if session.TotalTokens > 0 {
			proj.ActiveSessionCount++
			proj.TotalInputTokens += session.InputTokens
			proj.TotalCacheRead += session.CacheReadTokens
			proj.TotalOutput += session.OutputTokens
			proj.TotalTokens += session.TotalTokens
			proj.TotalCostUSD += session.CostUSD
			proj.TotalCostVND += session.CostVND
		}

		// Aggregate models per project
		pModels, pExists := projModelMap[projName]
		if !pExists {
			pModels = make(map[string]*ModelUsageStat)
			projModelMap[projName] = pModels
		}
		for _, mStat := range session.Models {
			pm, ok := pModels[mStat.ModelName]
			if !ok {
				pm = &ModelUsageStat{ModelName: mStat.ModelName}
				pModels[mStat.ModelName] = pm
			}
			pm.StepCount += mStat.StepCount
			pm.InputTokens += mStat.InputTokens
			pm.CacheReadTokens += mStat.CacheReadTokens
			pm.OutputTokens += mStat.OutputTokens
			pm.TotalTokens += mStat.TotalTokens
			pm.CostUSD += mStat.CostUSD
			pm.CostVND += mStat.CostVND
		}

		sCopy := *session
		sCopy.Steps = nil
		proj.Sessions = append(proj.Sessions, &sCopy)

		if session.TotalTokens > 0 {
			stats.ActiveTokenSessions++
			stats.TotalInputTokens += session.InputTokens
			stats.TotalCacheRead += session.CacheReadTokens
			stats.TotalOutput += session.OutputTokens
			stats.TotalTokens += session.TotalTokens
			stats.TotalCostUSD += session.CostUSD
			stats.TotalCostVND += session.CostVND

			stats.ModelDistribution[session.ModelName] += session.TotalTokens

			dateKey := session.CreatedAt.Format("2006-01-02")
			if !session.CreatedAt.IsZero() {
				dm, exists := dailyMap[dateKey]
				if !exists {
					dm = &DailyMetric{Date: dateKey}
					dailyMap[dateKey] = dm
				}
				dm.SessionCount++
				dm.InputTokens += session.InputTokens
				dm.CacheReadTokens += session.CacheReadTokens
				dm.OutputTokens += session.OutputTokens
				dm.TotalTokens += session.TotalTokens
				dm.CostUSD += session.CostUSD
				dm.CostVND += session.CostVND
			}
		}
	}

	if stats.TotalCacheRead+stats.TotalInputTokens > 0 {
		stats.OverallCacheHitRate = (float64(stats.TotalCacheRead) / float64(stats.TotalCacheRead+stats.TotalInputTokens)) * 100.0
	}

	// Process project metrics
	for _, proj := range projectMap {
		if proj.TotalCacheRead+proj.TotalInputTokens > 0 {
			proj.OverallCacheHitRate = (float64(proj.TotalCacheRead) / float64(proj.TotalCacheRead+proj.TotalInputTokens)) * 100.0
		}
		sort.Slice(proj.Sessions, func(i, j int) bool {
			return proj.Sessions[i].LastModified.After(proj.Sessions[j].LastModified)
		})
		if pModels, ok := projModelMap[proj.Name]; ok {
			for _, pm := range pModels {
				if pm.CacheReadTokens+pm.InputTokens > 0 {
					pm.OverallCacheHitRate = (float64(pm.CacheReadTokens) / float64(pm.CacheReadTokens+pm.InputTokens)) * 100.0
				}
				proj.Models = append(proj.Models, *pm)
			}
			sort.Slice(proj.Models, func(i, j int) bool {
				return proj.Models[i].TotalTokens > proj.Models[j].TotalTokens
			})
		}
		stats.Projects = append(stats.Projects, *proj)
	}

	sort.Slice(stats.Projects, func(i, j int) bool {
		if stats.Projects[i].TotalTokens == stats.Projects[j].TotalTokens {
			return stats.Projects[i].SessionCount > stats.Projects[j].SessionCount
		}
		return stats.Projects[i].TotalTokens > stats.Projects[j].TotalTokens
	})

	// Sort sessions by LastModified descending
	sort.Slice(allSessions, func(i, j int) bool {
		return allSessions[i].LastModified.After(allSessions[j].LastModified)
	})

	// Process daily metrics
	for _, dm := range dailyMap {
		if dm.CacheReadTokens+dm.InputTokens > 0 {
			dm.CacheHitRate = (float64(dm.CacheReadTokens) / float64(dm.CacheReadTokens+dm.InputTokens)) * 100.0
		}
		stats.DailyMetrics = append(stats.DailyMetrics, *dm)
	}
	sort.Slice(stats.DailyMetrics, func(i, j int) bool {
		return stats.DailyMetrics[i].Date < stats.DailyMetrics[j].Date
	})

	// Top recent sessions
	limit := 10
	if len(allSessions) < limit {
		limit = len(allSessions)
	}
	for i := 0; i < limit; i++ {
		// Clone without steps to keep payload compact
		sCopy := *allSessions[i]
		sCopy.Steps = nil
		stats.RecentSessions = append(stats.RecentSessions, sCopy)
	}

	return stats, allSessions, nil
}

// GetSessionDetail loads a specific session by ID with full step details.
func (s *Scanner) GetSessionDetail(sessionID string) (*SessionMetric, error) {
	metaMap := s.loadSummariesFromSQLite()
	transcriptPath := filepath.Join(s.dataDir, "brain", sessionID, ".system_generated", "logs", "transcript.jsonl")
	return ParseTranscriptFile(sessionID, transcriptPath, metaMap[sessionID])
}
