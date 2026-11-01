package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTranscriptFile(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "transcript.jsonl")

	sampleContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-31T12:00:00Z","content":"<USER_REQUEST>\nReview code and build dashboard\n</USER_REQUEST>\n<USER_SETTINGS_CHANGE>\nThe user changed setting ` + "`Model Selection`" + ` from None to Gemini 3.8 Flash (High).\n</USER_SETTINGS_CHANGE>"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-31T12:00:02Z","input_tokens":1000,"cache_read_tokens":0,"output_tokens":150,"thinking":"Analyzing prompt...","tool_calls":[{"name":"run_command","args":{"toolSummary":"Check git status"}}]}
{"step_index":2,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":"2026-10-31T12:00:04Z","content":"All files clean"}
{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-31T12:00:06Z","input_tokens":200,"cache_read_tokens":900,"output_tokens":80,"thinking":"Done!"}
`
	if err := os.WriteFile(logPath, []byte(sampleContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	session, err := ParseTranscriptFile("test-session-1234", logPath, conversationMeta{
		Title: "Test Title from DB",
	})
	if err != nil {
		t.Fatalf("ParseTranscriptFile returned unexpected error: %v", err)
	}

	if session.Title != "Test Title from DB" {
		t.Errorf("expected title 'Test Title from DB', got '%s'", session.Title)
	}
	if session.InputTokens != 1200 {
		t.Errorf("expected InputTokens 1200, got %d", session.InputTokens)
	}
	if session.CacheReadTokens != 900 {
		t.Errorf("expected CacheReadTokens 900, got %d", session.CacheReadTokens)
	}
	if session.OutputTokens != 230 {
		t.Errorf("expected OutputTokens 230, got %d", session.OutputTokens)
	}
	if session.TotalTokens != 2330 {
		t.Errorf("expected TotalTokens 2330, got %d", session.TotalTokens)
	}

	// CacheHitRate = 900 / (900 + 1200) * 100 = 900 / 2100 * 100 ~= 42.857
	expectedRate := (900.0 / 2100.0) * 100.0
	diff := session.CacheHitRate - expectedRate
	if diff < -0.01 || diff > 0.01 {
		t.Errorf("expected CacheHitRate ~%.2f%%, got %.2f%%", expectedRate, session.CacheHitRate)
	}

	if session.StepCount != 4 {
		t.Errorf("expected 4 steps, got %d", session.StepCount)
	}
	if session.CostUSD <= 0 {
		t.Errorf("expected positive CostUSD, got %f", session.CostUSD)
	}
	if session.CostVND <= 0 {
		t.Errorf("expected positive CostVND, got %f", session.CostVND)
	}
}

func TestCalculateCost(t *testing.T) {
	// 1,000,000 input, 1,000,000 cache read, 1,000,000 output with Gemini Flash
	// Input: 0.10, Cache: 0.025, Output: 0.40 => Total 0.525 USD
	costUSD, costVND := CalculateCost("Gemini 3.8 Flash", 1_000_000, 1_000_000, 1_000_000)
	expectedUSD := 0.10 + 0.025 + 0.40
	if costUSD != expectedUSD {
		t.Errorf("expected USD %f, got %f", expectedUSD, costUSD)
	}
	if costVND != expectedUSD*DefaultUSDToVNDRate {
		t.Errorf("expected VND %f, got %f", expectedUSD*DefaultUSDToVNDRate, costVND)
	}
}

func TestShortSessionIDAndRuneTruncation(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "transcript.jsonl")

	sampleContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-31T12:00:00Z","content":"Không có tag user request"}`
	if err := os.WriteFile(logPath, []byte(sampleContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Short ID with length < 8 (should not panic)
	session, err := ParseTranscriptFile("dev", logPath, conversationMeta{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.Title != "Conversation dev" {
		t.Errorf("expected 'Conversation dev', got '%s'", session.Title)
	}

	// Truncate unicode runes
	vietnamese := "Kiểm tra độ dài chuỗi tiếng Việt có dấu rất dài cần cắt ngắn an toàn"
	truncated := truncateRunes(vietnamese, 20)
	if len([]rune(truncated)) > 20 {
		t.Errorf("expected max 20 runes, got %d", len([]rune(truncated)))
	}
}

func TestModelExtractionAndNormalization(t *testing.T) {
	// 1. Verify Normalization
	cases := []struct {
		raw      string
		expected string
	}{
		{"gemini-3.5-flash-low", "Gemini 3.5 Flash (Low)"},
		{"gemini-3.8-flash-high", "Gemini 3.8 Flash (High)"},
		{"claude-opus-4-6-thinking", "Claude Opus 4.6 (Thinking)"},
		{"Claude Sonnet 4.6 (Thinking)", "Claude Sonnet 4.6 (Thinking)"},
		{"gemini-default", "Gemini 3.5 Flash (Medium)"},
		{"gemini-4-argon-medium", "Gemini 4 Argon (Medium)"},
		{"gemini-3.1-pro-low", "Gemini 3.1 Pro (Low)"},
		{"GPT-OSS 120B (Medium)", "GPT-OSS 120B (Medium)"},
	}

	for _, c := range cases {
		got := NormalizeModelName(c.raw)
		if got != c.expected {
			t.Errorf("NormalizeModelName(%q) = %q, expected %q", c.raw, got, c.expected)
		}
	}

	// 2. Verify Transcript with false positive "from ... to ..." sentences
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "transcript.jsonl")

	sampleContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-31T12:00:00Z","content":"<USER_REQUEST>\nLet us test changing from A to continue. And switch from X to nil if you wish to use default.\n</USER_REQUEST>\n<USER_SETTINGS_CHANGE>\nThe user changed setting ` + "`Model Selection`" + ` from None to Claude Sonnet 4.6 (Thinking). No need to comment on this change if the user doesn't ask about it.\n</USER_SETTINGS_CHANGE>"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-31T12:00:02Z","input_tokens":500,"output_tokens":100,"content":"I shifted from old to the email field."}
`
	if err := os.WriteFile(logPath, []byte(sampleContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	session, err := ParseTranscriptFile("model-test", logPath, conversationMeta{})
	if err != nil {
		t.Fatalf("ParseTranscriptFile error: %v", err)
	}

	if session.ModelName != "Claude Sonnet 4.6 (Thinking)" {
		t.Errorf("expected ModelName 'Claude Sonnet 4.6 (Thinking)', got %q", session.ModelName)
	}

	// Ensure false-positive strings were NEVER recorded in session.Models
	for _, m := range session.Models {
		if m.ModelName == "continue" || m.ModelName == "nil if you wish to use default" || m.ModelName == "the email field" {
			t.Errorf("junk string %q mistakenly recorded as model!", m.ModelName)
		}
	}
}

func TestSubagentDetectionAndTitleExtraction(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "transcript.jsonl")

	sampleContent := `{"step_index":0,"source":"SYSTEM","type":"SYSTEM_MESSAGE","status":"DONE","created_at":"2026-10-31T12:00:00Z","content":"<SYSTEM_MESSAGE>\n[Message] timestamp=2026-10-31T12:00:00Z sender=parent-123 priority=MESSAGE_PRIORITY_HIGH content=Hãy độc lập đánh giá Code Review cho tính năng Toolbar chú thích màn hình.\n</SYSTEM_MESSAGE>"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-31T12:00:05Z","input_tokens":300,"output_tokens":50,"content":"I have completed the review."}
`
	if err := os.WriteFile(logPath, []byte(sampleContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	meta := conversationMeta{
		AgentName:            "reviewer",
		ParentConversationID: "parent-123",
		NestingDepth:         1,
	}

	session, err := ParseTranscriptFile("sub-008b89f3", logPath, meta)
	if err != nil {
		t.Fatalf("ParseTranscriptFile error: %v", err)
	}

	if !session.IsSubagent {
		t.Errorf("expected session.IsSubagent to be true")
	}
	if session.AgentName != "reviewer" {
		t.Errorf("expected AgentName 'reviewer', got %q", session.AgentName)
	}
	if session.ParentConversationID != "parent-123" {
		t.Errorf("expected ParentConversationID 'parent-123', got %q", session.ParentConversationID)
	}
	if session.NestingDepth != 1 {
		t.Errorf("expected NestingDepth 1, got %d", session.NestingDepth)
	}
	if !strings.Contains(session.Title, "Hãy độc lập đánh giá Code Review") {
		t.Errorf("expected Title to contain task description, got %q", session.Title)
	}
}

func TestMultiEnvironmentAndToolStats(t *testing.T) {
	tempParent := t.TempDir()
	appDir := filepath.Join(tempParent, "antigravity")
	cliDir := filepath.Join(tempParent, "antigravity-cli")

	// Create app session
	appLogDir := filepath.Join(appDir, "brain", "sess-app", ".system_generated", "logs")
	if err := os.MkdirAll(appLogDir, 0755); err != nil {
		t.Fatalf("failed to create appLogDir: %v", err)
	}
	appContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-31T12:00:00Z","content":"<USER_REQUEST>Build app</USER_REQUEST>"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-31T12:00:01Z","input_tokens":100,"cache_read_tokens":400,"output_tokens":50,"tool_calls":[{"name":"run_command","args":{}},{"name":"read_file","args":{}}]}`
	if err := os.WriteFile(filepath.Join(appLogDir, "transcript.jsonl"), []byte(appContent), 0644); err != nil {
		t.Fatalf("failed to write app transcript: %v", err)
	}

	// Create CLI session
	cliLogDir := filepath.Join(cliDir, "brain", "sess-cli", ".system_generated", "logs")
	if err := os.MkdirAll(cliLogDir, 0755); err != nil {
		t.Fatalf("failed to create cliLogDir: %v", err)
	}
	cliContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-31T13:00:00Z","content":"<USER_REQUEST>Run CLI command</USER_REQUEST>"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-31T13:00:01Z","input_tokens":200,"cache_read_tokens":600,"output_tokens":80,"tool_calls":[{"name":"run_command","args":{}}]}`
	if err := os.WriteFile(filepath.Join(cliLogDir, "transcript.jsonl"), []byte(cliContent), 0644); err != nil {
		t.Fatalf("failed to write cli transcript: %v", err)
	}

	scanner := NewScanner(appDir)
	stats, sessions, err := scanner.ScanAll()
	if err != nil {
		t.Fatalf("ScanAll failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions across environments, got %d", len(sessions))
	}

	// Check Tools stats
	if len(stats.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(stats.Tools))
	}
	toolNames := make(map[string]bool)
	for _, tool := range stats.Tools {
		toolNames[tool.Name] = true
	}
	if !toolNames["Antigravity App"] || !toolNames["Antigravity CLI"] {
		t.Errorf("expected Antigravity App and Antigravity CLI in tools, got %v", toolNames)
	}

	// Check Agent Tools stats
	agentToolMap := make(map[string]int64)
	for _, at := range stats.AgentTools {
		agentToolMap[at.Name] = at.CallCount
	}
	if agentToolMap["run_command"] != 2 {
		t.Errorf("expected run_command call_count 2, got %d", agentToolMap["run_command"])
	}
	if agentToolMap["read_file"] != 1 {
		t.Errorf("expected read_file call_count 1, got %d", agentToolMap["read_file"])
	}

	// Check System Models stats
	if len(stats.Models) == 0 {
		t.Errorf("expected non-empty system Models stats")
	}

	// Check GetSessionDetail for CLI session
	cliDetail, err := scanner.GetSessionDetail("sess-cli")
	if err != nil {
		t.Fatalf("GetSessionDetail for sess-cli failed: %v", err)
	}
	if cliDetail.ClientTool != "Antigravity CLI" {
		t.Errorf("expected ClientTool 'Antigravity CLI', got %q", cliDetail.ClientTool)
	}
}
