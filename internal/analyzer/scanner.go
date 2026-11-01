package analyzer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type environmentTarget struct {
	Name    string // "Antigravity App", "Antigravity CLI", "Antigravity IDE"
	DataDir string
	Icon    string // "desktop", "terminal", "code"
}

// Scanner reads Antigravity conversation logs and extracts token metrics across environments.
type Scanner struct {
	primaryDir   string
	environments []environmentTarget
	sessionMap   map[string]environmentTarget
	mapMutex     sync.RWMutex
}

func discoverEnvironments(primaryDir string) []environmentTarget {
	var envs []environmentTarget
	seen := make(map[string]bool)

	addEnv := func(path string, fallbackName, icon string) {
		clean := filepath.Clean(path)
		if seen[clean] {
			return
		}
		if _, err := os.Stat(clean); err == nil {
			seen[clean] = true
			name := fallbackName
			base := filepath.Base(clean)
			if strings.Contains(base, "-cli") {
				name = "Antigravity CLI"
				icon = "terminal"
			} else if strings.Contains(base, "-ide") {
				name = "Antigravity IDE"
				icon = "code"
			} else if strings.Contains(base, "antigravity") {
				name = "Antigravity App"
				icon = "desktop"
			}
			envs = append(envs, environmentTarget{
				Name:    name,
				DataDir: clean,
				Icon:    icon,
			})
		}
	}

	// 1. Add primaryDir
	addEnv(primaryDir, "Antigravity App", "desktop")

	// 2. Discover sibling environments if primaryDir is an Antigravity environment
	parentDir := filepath.Dir(primaryDir)
	baseName := filepath.Base(primaryDir)
	if strings.Contains(baseName, "antigravity") || strings.HasSuffix(parentDir, ".gemini") {
		addEnv(filepath.Join(parentDir, "antigravity"), "Antigravity App", "desktop")
		addEnv(filepath.Join(parentDir, "antigravity-cli"), "Antigravity CLI", "terminal")
		addEnv(filepath.Join(parentDir, "antigravity-ide"), "Antigravity IDE", "code")
	}

	return envs
}

// NewScanner creates a new Scanner instance with resolved data directories.
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
	envs := discoverEnvironments(dataDir)
	return &Scanner{
		primaryDir:   dataDir,
		environments: envs,
		sessionMap:   make(map[string]environmentTarget),
	}
}

// DataDir returns the primary directory path.
func (s *Scanner) DataDir() string {
	return s.primaryDir
}

// Environments returns the discovered environment targets.
func (s *Scanner) Environments() []environmentTarget {
	return s.environments
}

func categorizeTool(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "command") || strings.Contains(lower, "bash") || strings.Contains(lower, "cmd") || strings.Contains(lower, "exec"):
		return "Execution"
	case strings.Contains(lower, "file") || strings.Contains(lower, "dir"):
		return "Filesystem"
	case strings.Contains(lower, "search") || strings.Contains(lower, "grep") || strings.Contains(lower, "find") || strings.Contains(lower, "browse"):
		return "Search"
	case strings.Contains(lower, "mcp") || strings.Contains(lower, "resource"):
		return "MCP"
	case strings.Contains(lower, "subagent") || strings.Contains(lower, "message") || strings.Contains(lower, "task") || strings.Contains(lower, "schedule"):
		return "Orchestration"
	case strings.Contains(lower, "image"):
		return "Media"
	default:
		return "Interaction"
	}
}

func resolveToolGroup(name string) string {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "mcp_chrome_devtools_") {
		return "Chrome DevTools MCP"
	}
	if strings.HasPrefix(lower, "mcp_sequentialthinking_") {
		return "Sequential Thinking MCP"
	}
	if strings.HasPrefix(lower, "mcp_") {
		parts := strings.Split(name, "_")
		if len(parts) >= 3 {
			server := parts[1]
			if len(server) > 0 {
				return strings.ToUpper(server[:1]) + server[1:] + " MCP"
			}
		}
		return "MCP Tools"
	}
	return "Native Tools"
}

// ScanAll scans all conversations across all environments and compiles aggregate statistics.
func (s *Scanner) ScanAll() (*GlobalStats, []*SessionMetric, error) {
	type scanJob struct {
		id       string
		path     string
		metaInfo conversationMeta
		env      environmentTarget
	}

	var jobs []scanJob
	newSessionMap := make(map[string]environmentTarget)

	for _, env := range s.environments {
		metaMap := loadSummariesFromSQLite(env.DataDir)
		brainDir := filepath.Join(env.DataDir, "brain")
		entries, err := os.ReadDir(brainDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				sessID := entry.Name()
				transcriptPath := filepath.Join(brainDir, sessID, ".system_generated", "logs", "transcript.jsonl")
				if _, err := os.Stat(transcriptPath); err == nil {
					meta := metaMap[sessID]
					meta.ClientTool = env.Name
					jobs = append(jobs, scanJob{
						id:       sessID,
						path:     transcriptPath,
						metaInfo: meta,
						env:      env,
					})
					newSessionMap[sessID] = env
				}
			}
		}
	}

	s.mapMutex.Lock()
	s.sessionMap = newSessionMap
	s.mapMutex.Unlock()

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
					session.ClientTool = job.env.Name
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
		Models:            make([]ModelUsageStat, 0),
		Tools:             make([]ClientToolStat, 0),
		AgentTools:        make([]AgentToolStat, 0),
		Projects:          make([]ProjectStats, 0),
	}

	dailyMap := make(map[string]*DailyMetric)
	projectMap := make(map[string]*ProjectStats)
	projModelMap := make(map[string]map[string]*ModelUsageStat)
	systemModelMap := make(map[string]*ModelUsageStat)
	agentToolCounts := make(map[string]int64)

	// Tool/Environment aggregations
	type toolAccumulator struct {
		stat     ClientToolStat
		projects map[string]bool
		lastTime time.Time
	}
	toolAccMap := make(map[string]*toolAccumulator)
	for _, env := range s.environments {
		toolAccMap[env.Name] = &toolAccumulator{
			stat: ClientToolStat{
				Name:    env.Name,
				Icon:    env.Icon,
				DataDir: env.DataDir,
			},
			projects: make(map[string]bool),
		}
	}

	for session := range resultsChan {
		allSessions = append(allSessions, session)
		stats.TotalSteps += session.StepCount

		// Tally Agent Tool calls
		for _, st := range session.Steps {
			for _, tc := range st.ToolCalls {
				if tc.Name != "" {
					agentToolCounts[tc.Name]++
				}
			}
		}

		// Tool environment tally
		tAcc, tExists := toolAccMap[session.ClientTool]
		if !tExists {
			tAcc = &toolAccumulator{
				stat: ClientToolStat{
					Name: session.ClientTool,
					Icon: "desktop",
				},
				projects: make(map[string]bool),
			}
			toolAccMap[session.ClientTool] = tAcc
		}
		tAcc.stat.SessionCount++
		if session.LastModified.After(tAcc.lastTime) {
			tAcc.lastTime = session.LastModified
		}
		if session.TotalTokens > 0 {
			tAcc.stat.InputTokens += session.InputTokens
			tAcc.stat.CacheReadTokens += session.CacheReadTokens
			tAcc.stat.OutputTokens += session.OutputTokens
			tAcc.stat.TotalTokens += session.TotalTokens
			tAcc.stat.CostUSD += session.CostUSD
			tAcc.stat.CostVND += session.CostVND
		}
		if session.ProjectName != "" {
			tAcc.projects[session.ProjectName] = true
		}

		// System-wide model aggregation
		for _, mStat := range session.Models {
			sm, ok := systemModelMap[mStat.ModelName]
			if !ok {
				sm = &ModelUsageStat{ModelName: mStat.ModelName}
				systemModelMap[mStat.ModelName] = sm
			}
			sm.StepCount += mStat.StepCount
			sm.InputTokens += mStat.InputTokens
			sm.CacheReadTokens += mStat.CacheReadTokens
			sm.OutputTokens += mStat.OutputTokens
			sm.TotalTokens += mStat.TotalTokens
			sm.CostUSD += mStat.CostUSD
			sm.CostVND += mStat.CostVND
		}

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

	// Finalize Tools stats
	for _, tAcc := range toolAccMap {
		if tAcc.stat.CacheReadTokens+tAcc.stat.InputTokens > 0 {
			tAcc.stat.CacheHitRate = (float64(tAcc.stat.CacheReadTokens) / float64(tAcc.stat.CacheReadTokens+tAcc.stat.InputTokens)) * 100.0
		}
		tAcc.stat.ActiveProjects = len(tAcc.projects)
		if !tAcc.lastTime.IsZero() {
			tAcc.stat.LastActivity = tAcc.lastTime.Format("2006-01-02 15:04:05")
		}
		stats.Tools = append(stats.Tools, tAcc.stat)
	}
	sort.Slice(stats.Tools, func(i, j int) bool {
		return stats.Tools[i].TotalTokens > stats.Tools[j].TotalTokens
	})

	// Finalize System-wide Models stats
	for _, sm := range systemModelMap {
		if sm.CacheReadTokens+sm.InputTokens > 0 {
			sm.OverallCacheHitRate = (float64(sm.CacheReadTokens) / float64(sm.CacheReadTokens+sm.InputTokens)) * 100.0
		}
		stats.Models = append(stats.Models, *sm)
	}
	sort.Slice(stats.Models, func(i, j int) bool {
		return stats.Models[i].TotalTokens > stats.Models[j].TotalTokens
	})

	// Finalize Agent Tools stats
	for name, count := range agentToolCounts {
		stats.AgentTools = append(stats.AgentTools, AgentToolStat{
			Name:        name,
			CallCount:   count,
			Category:    categorizeTool(name),
			ServerGroup: resolveToolGroup(name),
		})
	}
	sort.Slice(stats.AgentTools, func(i, j int) bool {
		return stats.AgentTools[i].CallCount > stats.AgentTools[j].CallCount
	})

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
		sCopy := *allSessions[i]
		sCopy.Steps = nil
		stats.RecentSessions = append(stats.RecentSessions, sCopy)
	}

	return stats, allSessions, nil
}

// GetSessionDetail loads a specific session by ID with full step details across environments.
func (s *Scanner) GetSessionDetail(sessionID string) (*SessionMetric, error) {
	s.mapMutex.RLock()
	targetEnv, found := s.sessionMap[sessionID]
	s.mapMutex.RUnlock()

	if found {
		metaMap := loadSummariesFromSQLite(targetEnv.DataDir)
		meta := metaMap[sessionID]
		meta.ClientTool = targetEnv.Name
		transcriptPath := filepath.Join(targetEnv.DataDir, "brain", sessionID, ".system_generated", "logs", "transcript.jsonl")
		return ParseTranscriptFile(sessionID, transcriptPath, meta)
	}

	// Fallback: check each environment in order
	for _, env := range s.environments {
		transcriptPath := filepath.Join(env.DataDir, "brain", sessionID, ".system_generated", "logs", "transcript.jsonl")
		if _, err := os.Stat(transcriptPath); err == nil {
			metaMap := loadSummariesFromSQLite(env.DataDir)
			meta := metaMap[sessionID]
			meta.ClientTool = env.Name
			return ParseTranscriptFile(sessionID, transcriptPath, meta)
		}
	}

	// Default fallback to primaryDir
	metaMap := loadSummariesFromSQLite(s.primaryDir)
	meta := metaMap[sessionID]
	meta.ClientTool = "Antigravity App"
	transcriptPath := filepath.Join(s.primaryDir, "brain", sessionID, ".system_generated", "logs", "transcript.jsonl")
	return ParseTranscriptFile(sessionID, transcriptPath, meta)
}
