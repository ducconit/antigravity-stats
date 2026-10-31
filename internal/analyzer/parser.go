package analyzer

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

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
