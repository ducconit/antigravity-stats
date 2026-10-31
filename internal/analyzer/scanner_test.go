package analyzer

import (
	"os"
	"path/filepath"
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
