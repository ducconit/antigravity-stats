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
	userRequestRegex  = regexp.MustCompile(`(?s)<USER_REQUEST>(.*?)</USER_REQUEST>`)
	subagentTaskRegex = regexp.MustCompile(`(?i)content=([^\r\n]+)`)
	modelSettingRegex = regexp.MustCompile("`Model Selection`\\s+from\\s+.*?\\s+to\\s+([A-Za-z0-9. ()-]+?)(?:\\.\\s*No need|\\.\\r?\\n|\\.</USER_SETTINGS_CHANGE>|\\.$)")
)

// NormalizeModelName maps raw model strings or internal IDs to clean, standardized names.
func NormalizeModelName(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "Gemini 3.5 Flash (Medium)"
	}
	lower := strings.ToLower(trimmed)

	switch {
	case strings.Contains(lower, "claude-opus") || strings.Contains(lower, "claude opus"):
		return "Claude Opus 4.6 (Thinking)"
	case strings.Contains(lower, "claude-sonnet") || strings.Contains(lower, "claude sonnet"):
		return "Claude Sonnet 4.6 (Thinking)"
	case strings.Contains(lower, "gpt-oss"):
		return "GPT-OSS 120B (Medium)"
	case strings.Contains(lower, "gemini-4-argon") || strings.Contains(lower, "gemini 4 argon"):
		if strings.Contains(lower, "high") {
			return "Gemini 4 Argon (High)"
		}
		return "Gemini 4 Argon (Medium)"
	case strings.Contains(lower, "gemini-3.8-flash") || strings.Contains(lower, "gemini 3.8 flash"):
		if strings.Contains(lower, "low") {
			return "Gemini 3.8 Flash (Low)"
		}
		if strings.Contains(lower, "medium") {
			return "Gemini 3.8 Flash (Medium)"
		}
		return "Gemini 3.8 Flash (High)"
	case strings.Contains(lower, "gemini-3.7-flash") || strings.Contains(lower, "gemini 3.7 flash"):
		return "Gemini 3.7 Flash (High)"
	case strings.Contains(lower, "gemini-3.6-flash") || strings.Contains(lower, "gemini 3.6 flash"):
		if strings.Contains(lower, "high") {
			return "Gemini 3.6 Flash (High)"
		}
		return "Gemini 3.6 Flash (Medium)"
	case strings.Contains(lower, "gemini-3.5-flash") || strings.Contains(lower, "gemini 3.5 flash"):
		if strings.Contains(lower, "low") {
			return "Gemini 3.5 Flash (Low)"
		}
		return "Gemini 3.5 Flash (Medium)"
	case strings.Contains(lower, "gemini-3.1-pro") || strings.Contains(lower, "gemini 3.1 pro"):
		return "Gemini 3.1 Pro (Low)"
	case strings.Contains(lower, "gemini-default") || strings.Contains(lower, "gemini-3-flash"):
		return "Gemini 3.5 Flash (Medium)"
	}

	validModelKeywords := []string{
		"gemini", "claude", "gpt", "deepseek", "chatgpt",
		"llama", "mistral", "qwen", "sonnet", "opus", "haiku",
		"flash", "pro", "argon",
	}
	for _, kw := range validModelKeywords {
		if strings.Contains(lower, kw) {
			trimmed = strings.Trim(trimmed, "`'\"")
			return trimmed
		}
	}

	return "Gemini 3.5 Flash (Medium)"
}

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

	// Determine initial model: prioritize meta.DBModel, then conversation SQLite database
	initialModel := meta.DBModel
	if initialModel == "" {
		dataDir := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filePath))))
		convDBPath := filepath.Join(dataDir, "conversations", sessionID+".db")
		initialModel = readModelFromConversationDB(convDBPath)
	}
	if initialModel == "" {
		initialModel = "Gemini 3.5 Flash (Medium)"
	} else {
		initialModel = NormalizeModelName(initialModel)
	}

	isSubagent := meta.ParentConversationID != "" || meta.NestingDepth > 0 || meta.AgentName != ""

	session := &SessionMetric{
		ID:                   sessionID,
		Title:                meta.Title,
		ProjectName:          projectName,
		Workspace:            wsPath,
		ModelName:            initialModel,
		Models:               make([]ModelUsageStat, 0),
		Steps:                make([]StepMetric, 0),
		AgentName:            meta.AgentName,
		ParentConversationID: meta.ParentConversationID,
		NestingDepth:         meta.NestingDepth,
		IsSubagent:           isSubagent,
		ClientTool:           meta.ClientTool,
	}

	currentModel := initialModel
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

				// Check for model selection / change (only inside USER_INPUT steps containing Model Selection)
				if raw.Type == "USER_INPUT" && raw.Content != "" && strings.Contains(raw.Content, "Model Selection") {
					if match := modelSettingRegex.FindStringSubmatch(raw.Content); len(match) > 1 {
						parsedModel := NormalizeModelName(match[1])
						if parsedModel != "" {
							currentModel = parsedModel
							session.ModelName = currentModel
						}
					}
				}

				// Extract Title if not present in SQLite
				if session.Title == "" && raw.Content != "" {
					if isSubagent {
						if raw.Type == "SYSTEM_MESSAGE" || strings.Contains(raw.Content, "content=") {
							if match := subagentTaskRegex.FindStringSubmatch(raw.Content); len(match) > 1 {
								taskText := strings.TrimSpace(match[1])
								taskText = strings.Trim(taskText, `"'`)
								if taskText != "" {
									session.Title = truncateRunes(taskText, 80)
								}
							}
						}
					} else {
						if match := userRequestRegex.FindStringSubmatch(raw.Content); len(match) > 1 {
							trimmed := strings.TrimSpace(match[1])
							lines := strings.Split(trimmed, "\n")
							for _, l := range lines {
								l = strings.TrimSpace(l)
								if l != "" && !strings.HasPrefix(l, "<") && !strings.HasPrefix(l, "<!--") {
									session.Title = truncateRunes(l, 100)
									break
								}
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
					Content:         raw.Content,
					Thinking:        raw.Thinking,
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
		shortID := sessionID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		if session.IsSubagent {
			tag := "Subagent"
			if session.AgentName != "" {
				tag = session.AgentName
			}
			session.Title = tag + " " + shortID
		} else {
			session.Title = "Conversation " + shortID
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
	if len(session.Models) > 0 {
		session.ModelName = session.Models[0].ModelName
	}

	session.TotalTokens = session.InputTokens + session.CacheReadTokens + session.OutputTokens
	if session.CacheReadTokens+session.InputTokens > 0 {
		session.CacheHitRate = (float64(session.CacheReadTokens) / float64(session.CacheReadTokens+session.InputTokens)) * 100.0
	}

	var totalCostUSD, totalCostVND float64
	for _, mStat := range session.Models {
		totalCostUSD += mStat.CostUSD
		totalCostVND += mStat.CostVND
	}
	if len(session.Models) > 0 {
		session.CostUSD = totalCostUSD
		session.CostVND = totalCostVND
	} else {
		session.CostUSD, session.CostVND = CalculateCost(session.ModelName, session.InputTokens, session.CacheReadTokens, session.OutputTokens)
	}

	return session, nil
}
