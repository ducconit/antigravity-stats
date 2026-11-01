package analyzer

import (
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type conversationMeta struct {
	Title                string
	Preview              string
	WorkspaceURIs        string
	Project              string
	LastModified         time.Time
	DBModel              string
	AgentName            string
	ParentConversationID string
	NestingDepth         int
}

var annotationTitleRegex = regexp.MustCompile(`title:\s*"([^"]+)"`)

func readTitleFromAnnotations(annotationsDir, id string) string {
	p := filepath.Join(annotationsDir, id+".pbtxt")
	content, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	m := annotationTitleRegex.FindSubmatch(content)
	if len(m) > 1 {
		return string(m[1])
	}
	return ""
}

// cleanPreviewTitle extracts a clean, readable one-line title from a preview string.
func cleanPreviewTitle(preview string) string {
	trimmed := strings.TrimSpace(preview)
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		// Strip markdown headers (###), lists (*, -), blockquotes (>), formatting marks
		l = strings.TrimLeft(l, "#*-> \t")
		l = strings.Trim(l, "`'\"")
		l = strings.TrimSpace(l)
		if l != "" {
			return truncateRunes(l, 100)
		}
	}
	return ""
}

// loadSummariesFromSQLite loads titles and metadata from conversation_summaries.db.
func loadSummariesFromSQLite(dataDir string) map[string]conversationMeta {
	metaMap := make(map[string]conversationMeta)
	dbPath := filepath.Join(dataDir, "conversation_summaries.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return metaMap
	}

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return metaMap
	}
	defer db.Close()

	annotationsDir := filepath.Join(dataDir, "annotations")

	rows, err := db.Query(`
		SELECT c.conversation_id,
		       COALESCE(NULLIF(c.title, ''), NULLIF(f.title, ''), ''),
		       c.preview, c.workspace_uris, c.last_modified_time,
		       COALESCE(f.project, ''),
		       COALESCE(c.agent_name, ''),
		       COALESCE(c.parent_conversation_id, ''),
		       COALESCE(c.nesting_depth, 0)
		FROM conversation_summaries c
		LEFT JOIN (
			SELECT conversation_id, project, title
			FROM conversations_fts
			WHERE title != '' OR project != ''
			GROUP BY conversation_id
		) f ON c.conversation_id = f.conversation_id
	`)
	if err != nil {
		rows, err = db.Query("SELECT conversation_id, title, preview, workspace_uris, last_modified_time, '', COALESCE(agent_name, ''), COALESCE(parent_conversation_id, ''), COALESCE(nesting_depth, 0) FROM conversation_summaries")
		if err != nil {
			return metaMap
		}
	}
	defer rows.Close()

	for rows.Next() {
		var id, title, preview, workspaceURIs, lastModStr, proj, agentName, parentID string
		var depth int
		if err := rows.Scan(&id, &title, &preview, &workspaceURIs, &lastModStr, &proj, &agentName, &parentID, &depth); err == nil {
			normalized := strings.Replace(lastModStr, " ", "T", 1)
			t, _ := time.Parse(time.RFC3339, normalized)
			if title == "" && preview != "" {
				title = cleanPreviewTitle(preview)
			}
			if title == "" {
				title = readTitleFromAnnotations(annotationsDir, id)
			}
			metaMap[id] = conversationMeta{
				Title:                title,
				Preview:              preview,
				WorkspaceURIs:        workspaceURIs,
				Project:              proj,
				LastModified:         t,
				AgentName:            agentName,
				ParentConversationID: parentID,
				NestingDepth:         depth,
			}
		}
	}
	return metaMap
}

var blobModelRegex = regexp.MustCompile(`(?:Gemini [A-Za-z0-9. ()-]+|Claude [A-Za-z0-9. ()-]+|GPT-[A-Za-z0-9. ()-]+|gemini-[a-z0-9.-]+|claude-[a-z0-9.-]+)`)

// extractModelFromBlob extracts and normalizes the model name from gen_metadata protobuf blob.
func extractModelFromBlob(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	matches := blobModelRegex.FindAll(data, -1)
	for _, m := range matches {
		s := strings.TrimSpace(string(m))
		sLow := strings.ToLower(s)
		if !strings.Contains(sLow, "http") &&
			!strings.Contains(sLow, "file:") &&
			!strings.Contains(sLow, "api") &&
			!strings.Contains(sLow, "cli") &&
			!strings.Contains(sLow, "code") &&
			!strings.Contains(sLow, "image") &&
			!strings.Contains(sLow, "model") &&
			!strings.Contains(sLow, "testcase") &&
			!strings.Contains(sLow, "desktop") &&
			!strings.Contains(sLow, "task") &&
			!strings.Contains(sLow, "walkthrough") {
			return NormalizeModelName(s)
		}
	}
	return ""
}

// readModelFromConversationDB reads the primary model from a conversation's SQLite database.
func readModelFromConversationDB(dbPath string) string {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return ""
	}
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return ""
	}
	defer db.Close()

	rows, err := db.Query("SELECT data FROM gen_metadata WHERE data IS NOT NULL ORDER BY idx DESC LIMIT 3")
	if err != nil {
		return ""
	}
	defer rows.Close()

	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err == nil {
			if m := extractModelFromBlob(data); m != "" {
				return m
			}
		}
	}
	return ""
}
