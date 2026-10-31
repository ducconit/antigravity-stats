package analyzer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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

// ScanAll scans all conversations in the data directory and compiles aggregate statistics.
func (s *Scanner) ScanAll() (*GlobalStats, []*SessionMetric, error) {
	metaMap := loadSummariesFromSQLite(s.dataDir)

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
	metaMap := loadSummariesFromSQLite(s.dataDir)
	transcriptPath := filepath.Join(s.dataDir, "brain", sessionID, ".system_generated", "logs", "transcript.jsonl")
	return ParseTranscriptFile(sessionID, transcriptPath, metaMap[sessionID])
}
